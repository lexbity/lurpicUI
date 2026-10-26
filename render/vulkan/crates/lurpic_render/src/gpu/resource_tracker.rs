//! Byte accounting for Vulkan allocations owned by the renderer.
//!
//! Charges are keyed by allocation ID, so equal-sized resources cannot retire
//! or acknowledge one another. The cap covers allocator-managed engine
//! allocations; swapchain and driver-private allocations remain external.

use std::collections::BTreeMap;
use std::sync::{Mutex, OnceLock};

use crate::RenderResult;

pub const DEFAULT_GPU_BUDGET_BYTES: u64 = 128 * 1024 * 1024;
const KIND_COUNT: usize = 6;
const MAX_TRACKED_ALLOCATIONS: usize = 65_536;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
#[repr(u8)]
pub enum ResourceKind {
    GpuResources = 0,
    AtlasTextures = 1,
    ImageTextures = 2,
    RingBuffers = 3,
    PipelineCache = 4,
    StagingBuffers = 5,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum State {
    Reserved,
    Live,
    Retiring,
}

#[derive(Clone, Copy, Debug)]
struct Record {
    bytes: u64,
    generation: u64,
    kind: ResourceKind,
    state: State,
}

#[derive(Debug)]
struct LedgerState {
    cap: u64,
    next_id: u64,
    generation: u64,
    records: BTreeMap<u64, Record>,
}

impl Default for LedgerState {
    fn default() -> Self {
        Self { cap: DEFAULT_GPU_BUDGET_BYTES, next_id: 1, generation: 0, records: BTreeMap::new() }
    }
}

#[derive(Debug, Default)]
pub struct ResourceLedger(Mutex<LedgerState>);

#[repr(C)]
#[derive(Clone, Copy, Debug, Default)]
pub struct ResourceSnapshot {
    pub live_bytes: [u64; KIND_COUNT],
    pub reserved_bytes: [u64; KIND_COUNT],
    pub retiring_bytes: [u64; KIND_COUNT],
    pub live_counts: [u64; KIND_COUNT],
    pub device_generation: u64,
    pub budget_bytes: u64,
}

impl ResourceSnapshot {
    pub fn total_live(&self) -> u64 { checked_sum(&self.live_bytes) }
    pub fn total_reserved(&self) -> u64 { checked_sum(&self.reserved_bytes) }
    pub fn total_retiring(&self) -> u64 { checked_sum(&self.retiring_bytes) }
    pub fn total_managed(&self) -> u64 {
        self.total_live().saturating_add(self.total_reserved()).saturating_add(self.total_retiring())
    }
}

fn checked_sum(values: &[u64]) -> u64 {
    values.iter().fold(0u64, |sum, value| sum.saturating_add(*value))
}

fn fail(message: impl Into<String>) -> (RenderResult, String) { (RenderResult::OutOfMemory, message.into()) }
fn contract(message: impl Into<String>) -> (RenderResult, String) { (RenderResult::Panic, message.into()) }

impl ResourceLedger {
    pub fn set_budget(&self, bytes: u64) -> Result<(), (RenderResult, String)> {
        if bytes == 0 { return Err(fail("Vulkan GPU budget must be positive")); }
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let used = state.records.values().try_fold(0u64, |sum, record| sum.checked_add(record.bytes))
            .ok_or_else(|| contract("Vulkan resource accounting overflow"))?;
        if bytes < used { return Err(fail(format!("Vulkan GPU budget {bytes} is below current charge {used}"))); }
        state.cap = bytes;
        Ok(())
    }

    pub fn bump_device_generation(&self) -> Result<u64, (RenderResult, String)> {
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        state.generation = state.generation.checked_add(1).ok_or_else(|| contract("Vulkan device generation exhausted"))?;
        Ok(state.generation)
    }

    pub fn reserve(&self, bytes: u64, kind: ResourceKind) -> Result<u64, (RenderResult, String)> {
        if bytes == 0 { return Err(fail("zero-byte Vulkan resource reservation")); }
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let used = state.records.values().try_fold(0u64, |sum, record| sum.checked_add(record.bytes))
            .ok_or_else(|| contract("Vulkan resource accounting overflow"))?;
        let next = used.checked_add(bytes).ok_or_else(|| fail("Vulkan resource reservation overflow"))?;
        if next > state.cap { return Err(fail(format!("Vulkan GPU budget exceeded: requested {bytes}, charged {used}, cap {}", state.cap))); }
        if state.records.len() >= MAX_TRACKED_ALLOCATIONS {
            return Err(fail(format!("Vulkan allocation record limit {MAX_TRACKED_ALLOCATIONS} reached")));
        }
        let id = state.next_id;
        state.next_id = state.next_id.checked_add(1).ok_or_else(|| contract("Vulkan allocation ID exhausted"))?;
        let generation = state.generation;
        state.records.insert(id, Record { bytes, generation, kind, state: State::Reserved });
        Ok(id)
    }

    pub fn publish(&self, id: u64, actual_bytes: u64) -> Result<(), (RenderResult, String)> {
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let record = state.records.get_mut(&id).ok_or_else(|| contract(format!("unknown Vulkan reservation {id}")))?;
        if record.state != State::Reserved { return Err(contract(format!("Vulkan allocation {id} published from {:?}", record.state))); }
        if actual_bytes == 0 || actual_bytes > record.bytes {
            return Err(fail(format!("Vulkan allocation {id} actual size {actual_bytes} exceeds reservation {}", record.bytes)));
        }
        record.bytes = actual_bytes;
        record.state = State::Live;
        Ok(())
    }

    pub fn reservation_generation(&self, id: u64) -> Result<u64, (RenderResult, String)> {
        let state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let record = state.records.get(&id).ok_or_else(|| contract(format!("unknown Vulkan reservation {id}")))?;
        if record.state != State::Reserved {
            return Err(contract(format!("Vulkan allocation {id} generation requested from {:?}", record.state)));
        }
        Ok(record.generation)
    }

    pub fn cancel(&self, id: u64) -> Result<(), (RenderResult, String)> { self.transition_remove(id, State::Reserved) }

    pub fn retire(&self, id: u64) -> Result<(), (RenderResult, String)> {
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let record = state.records.get_mut(&id).ok_or_else(|| contract(format!("unknown Vulkan allocation {id}")))?;
        if record.state != State::Live { return Err(contract(format!("Vulkan allocation {id} retired from {:?}", record.state))); }
        record.state = State::Retiring;
        Ok(())
    }

    pub fn acknowledge(&self, id: u64, generation: u64) -> Result<(), (RenderResult, String)> {
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let record = state.records.get(&id).ok_or_else(|| contract(format!("unknown Vulkan retirement {id}")))?;
        if record.state != State::Retiring { return Err(contract(format!("Vulkan allocation {id} acknowledged from {:?}", record.state))); }
        if record.generation != generation { return Err(contract(format!("stale Vulkan retirement generation for allocation {id}"))); }
        state.records.remove(&id);
        Ok(())
    }

    fn transition_remove(&self, id: u64, expected: State) -> Result<(), (RenderResult, String)> {
        let mut state = self.0.lock().map_err(|_| contract("Vulkan resource ledger poisoned"))?;
        let record = state.records.get(&id).ok_or_else(|| contract(format!("unknown Vulkan allocation {id}")))?;
        if record.state != expected { return Err(contract(format!("Vulkan allocation {id} released from {:?}", record.state))); }
        state.records.remove(&id);
        Ok(())
    }

    pub fn snapshot(&self) -> ResourceSnapshot {
        let state = self.0.lock().unwrap_or_else(|e| e.into_inner());
        let mut snapshot = ResourceSnapshot { device_generation: state.generation, budget_bytes: state.cap, ..Default::default() };
        for record in state.records.values() {
            let index = record.kind as usize;
            match record.state {
                State::Reserved => snapshot.reserved_bytes[index] = snapshot.reserved_bytes[index].saturating_add(record.bytes),
                State::Live => { snapshot.live_bytes[index] = snapshot.live_bytes[index].saturating_add(record.bytes); snapshot.live_counts[index] = snapshot.live_counts[index].saturating_add(1); },
                State::Retiring => snapshot.retiring_bytes[index] = snapshot.retiring_bytes[index].saturating_add(record.bytes),
            }
        }
        snapshot
    }
}

static LEDGER: OnceLock<ResourceLedger> = OnceLock::new();
pub fn ledger() -> &'static ResourceLedger { LEDGER.get_or_init(ResourceLedger::default) }
pub fn set_budget(bytes: u64) -> Result<(), (RenderResult, String)> { ledger().set_budget(bytes) }
pub fn bump_device_generation() -> Result<u64, (RenderResult, String)> { ledger().bump_device_generation() }
pub fn reserve(bytes: u64, kind: ResourceKind) -> Result<u64, (RenderResult, String)> { ledger().reserve(bytes, kind) }
pub fn publish(id: u64, actual_bytes: u64) -> Result<(), (RenderResult, String)> { ledger().publish(id, actual_bytes) }
pub fn reservation_generation(id: u64) -> Result<u64, (RenderResult, String)> { ledger().reservation_generation(id) }
pub fn cancel(id: u64) -> Result<(), (RenderResult, String)> { ledger().cancel(id) }
pub fn retire(id: u64) -> Result<(), (RenderResult, String)> { ledger().retire(id) }
pub fn acknowledge(id: u64, generation: u64) -> Result<(), (RenderResult, String)> { ledger().acknowledge(id, generation) }
pub fn snapshot() -> ResourceSnapshot { ledger().snapshot() }

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn allocation_identity_and_lifecycle_are_exact() {
        let ledger = ResourceLedger::default();
        ledger.set_budget(4096).unwrap();
        let a = ledger.reserve(1024, ResourceKind::GpuResources).unwrap();
        let b = ledger.reserve(1024, ResourceKind::GpuResources).unwrap();
        ledger.publish(a, 1024).unwrap();
        ledger.publish(b, 1024).unwrap();
        ledger.retire(a).unwrap();
        let generation = ledger.snapshot().device_generation;
        ledger.acknowledge(a, generation).unwrap();
        let snapshot = ledger.snapshot();
        assert_eq!(snapshot.live_bytes[0], 1024);
        assert_eq!(snapshot.retiring_bytes[0], 0);
        assert!(ledger.acknowledge(a, generation).is_err());
    }

    #[test]
    fn capacity_refusal_precedes_publication() {
        let ledger = ResourceLedger::default();
        ledger.set_budget(1024).unwrap();
        ledger.reserve(1024, ResourceKind::GpuResources).unwrap();
        assert!(ledger.reserve(1, ResourceKind::GpuResources).is_err());
    }

    #[test]
    fn replacement_peak_charges_old_new_and_staging_together() {
        let ledger = ResourceLedger::default();
        ledger.set_budget(7).unwrap();
        let old = ledger.reserve(2, ResourceKind::AtlasTextures).unwrap();
        ledger.publish(old, 2).unwrap();
        let new = ledger.reserve(4, ResourceKind::AtlasTextures).unwrap();
        let staging = ledger.reserve(1, ResourceKind::StagingBuffers).unwrap();
        assert_eq!(ledger.snapshot().total_managed(), 7);
        assert!(ledger.reserve(1, ResourceKind::AtlasTextures).is_err());

        ledger.publish(new, 4).unwrap();
        ledger.publish(staging, 1).unwrap();
        ledger.retire(old).unwrap();
        let generation = ledger.snapshot().device_generation;
        assert_eq!(ledger.snapshot().total_managed(), 7);
        ledger.acknowledge(old, generation).unwrap();
        ledger.retire(staging).unwrap();
        ledger.acknowledge(staging, generation).unwrap();
        assert_eq!(ledger.snapshot().total_live(), 4);
    }

    #[test]
    fn stale_generation_acknowledgment_keeps_charge() {
        let ledger = ResourceLedger::default();
        let allocation = ledger.reserve(64, ResourceKind::ImageTextures).unwrap();
        ledger.publish(allocation, 64).unwrap();
        ledger.retire(allocation).unwrap();
        let generation = ledger.snapshot().device_generation;
        ledger.bump_device_generation().unwrap();
        assert!(ledger.acknowledge(allocation, generation + 1).is_err());
        assert_eq!(ledger.snapshot().total_retiring(), 64);
        ledger.acknowledge(allocation, generation).unwrap();
    }

    #[test]
    fn reservation_keeps_its_creation_generation() {
        let ledger = ResourceLedger::default();
        let old_generation = ledger.reserve(64, ResourceKind::ImageTextures).unwrap();
        let generation = ledger.reservation_generation(old_generation).unwrap();
        ledger.bump_device_generation().unwrap();
        ledger.publish(old_generation, 64).unwrap();
        ledger.retire(old_generation).unwrap();
        assert_eq!(ledger.reservation_generation(old_generation).unwrap_err().0, RenderResult::Panic);
        assert!(ledger.acknowledge(old_generation, generation + 1).is_err());
        assert_eq!(ledger.snapshot().total_retiring(), 64);
        ledger.acknowledge(old_generation, generation).unwrap();
    }
}
