package assets

// waitingOn maps leaf AssetID to config AssetIDs waiting on it.
type waitingOn map[AssetID][]AssetID

// ConfigDependencyTree exposes runtime config dependency lookup.
type ConfigDependencyTree interface {
	ConfigNode(id AssetID) *ConfigNode
}

// ConfigNode describes one runtime config asset and its leaf dependencies.
type ConfigNode struct {
	ID   AssetID
	Path string
	Deps []AssetID
}

// SetDependencyTree installs the runtime dependency tree used by config scheduling.
func (m *managerImpl) SetDependencyTree(tree ConfigDependencyTree) {
	if m == nil {
		return
	}
	m.depTree = tree
}

func (m *managerImpl) scheduleConfig(id AssetID, path string) {
	if m == nil || m.depTree == nil || m.registry == nil {
		return
	}
	node := m.depTree.ConfigNode(id)
	if node == nil {
		return
	}
	if path == "" {
		path = node.Path
	}

	for _, depID := range node.Deps {
		depEntry := m.registry.Get(depID)
		if depEntry == nil || depEntry.LODHandles[0] == nil {
			m.enqueueWaiting(depID, id)
			return
		}
	}

	m.scheduleLOD(id, path, AssetTypeConfig, 0)
}

func (m *managerImpl) drainWaitingForLeaf(readyID AssetID) {
	if m == nil || m.depTree == nil {
		return
	}

	m.mu.Lock()
	pending := append([]AssetID(nil), m.waiting[readyID]...)
	delete(m.waiting, readyID)
	m.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	for _, configID := range pending {
		node := m.depTree.ConfigNode(configID)
		if node == nil {
			continue
		}
		m.scheduleConfig(configID, node.Path)
	}
}

func (m *managerImpl) enqueueWaiting(depID, configID AssetID) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.waiting[depID] {
		if existing == configID {
			return
		}
	}
	m.waiting[depID] = append(m.waiting[depID], configID)
}
