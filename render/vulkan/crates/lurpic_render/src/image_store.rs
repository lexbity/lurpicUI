use std::collections::HashMap;
use std::sync::{Mutex, OnceLock};

use crate::RenderResult;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ImageFormat {
    Rgba8 = 0,
    Bgra8 = 1,
}

#[derive(Clone, Debug)]
pub struct ImageBitmap {
    pub width: u32,
    pub height: u32,
    pub pixels: Vec<u8>,
}

#[derive(Clone, Debug)]
struct StoredImage {
    bitmap: ImageBitmap,
}

#[derive(Default)]
struct ImageStore {
    next_handle: u64,
    entries: HashMap<u64, StoredImage>,
    create_count: usize,
    destroy_count: usize,
}

impl ImageStore {
    fn new() -> Self {
        Self {
            next_handle: 1,
            entries: HashMap::new(),
            create_count: 0,
            destroy_count: 0,
        }
    }

    fn create(
        &mut self,
        pixels: &[u8],
        width: u32,
        height: u32,
        stride: u32,
        format: ImageFormat,
    ) -> Result<u64, (RenderResult, String)> {
        if width == 0 || height == 0 {
            return Err((
                RenderResult::InitFailed,
                "image dimensions are zero".to_string(),
            ));
        }
        let row_bytes = width as usize * 4;
        let stride = stride as usize;
        if stride < row_bytes {
            return Err((
                RenderResult::InitFailed,
                "image stride is smaller than width".to_string(),
            ));
        }
        let expected = stride.checked_mul(height as usize).ok_or((
            RenderResult::OutOfMemory,
            "image byte count overflow".to_string(),
        ))?;
        if pixels.len() < expected {
            return Err((
                RenderResult::InitFailed,
                "image pixel buffer is truncated".to_string(),
            ));
        }

        let mut rgba = vec![0u8; width as usize * height as usize * 4];
        for y in 0..height as usize {
            let src_row = &pixels[y * stride..y * stride + row_bytes];
            let dst_row = &mut rgba[y * row_bytes..(y + 1) * row_bytes];
            match format {
                ImageFormat::Rgba8 => dst_row.copy_from_slice(src_row),
                ImageFormat::Bgra8 => {
                    for x in 0..width as usize {
                        let src = &src_row[x * 4..x * 4 + 4];
                        let dst = &mut dst_row[x * 4..x * 4 + 4];
                        dst[0] = src[2];
                        dst[1] = src[1];
                        dst[2] = src[0];
                        dst[3] = src[3];
                    }
                }
            }
        }

        let handle = self.next_handle;
        self.next_handle += 1;
        self.entries.insert(
            handle,
            StoredImage {
                bitmap: ImageBitmap {
                    width,
                    height,
                    pixels: rgba,
                },
            },
        );
        self.create_count += 1;
        Ok(handle)
    }

    fn destroy(&mut self, handle: u64) -> Result<(), (RenderResult, String)> {
        if self.entries.remove(&handle).is_some() {
            self.destroy_count += 1;
            Ok(())
        } else {
            Err((
                RenderResult::InvalidHandle,
                format!("image handle {} does not exist", handle),
            ))
        }
    }

    fn lookup(&self, handle: u64) -> Option<ImageBitmap> {
        self.entries.get(&handle).map(|entry| entry.bitmap.clone())
    }

    fn reset(&mut self) {
        self.entries.clear();
        self.next_handle = 1;
        self.create_count = 0;
        self.destroy_count = 0;
    }
}

static IMAGE_STORE: OnceLock<Mutex<ImageStore>> = OnceLock::new();

fn store() -> &'static Mutex<ImageStore> {
    IMAGE_STORE.get_or_init(|| Mutex::new(ImageStore::new()))
}

pub fn create_image(
    pixels: &[u8],
    width: u32,
    height: u32,
    stride: u32,
    format: ImageFormat,
) -> Result<u64, (RenderResult, String)> {
    let mut store = store().lock().expect("image store mutex poisoned");
    store.create(pixels, width, height, stride, format)
}

pub fn destroy_image(handle: u64) -> Result<(), (RenderResult, String)> {
    let mut store = store().lock().expect("image store mutex poisoned");
    store.destroy(handle)
}

pub fn lookup_image(handle: u64) -> Option<ImageBitmap> {
    let store = store().lock().expect("image store mutex poisoned");
    store.lookup(handle)
}

pub fn image_stats() -> (usize, usize) {
    let store = store().lock().expect("image store mutex poisoned");
    (store.create_count, store.destroy_count)
}

pub fn reset_images() {
    let mut store = store().lock().expect("image store mutex poisoned");
    store.reset();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn create_lookup_and_destroy_image() {
        reset_images();
        let handle = create_image(&[255, 0, 0, 255], 1, 1, 4, ImageFormat::Rgba8).expect("create");
        let image = lookup_image(handle).expect("lookup");
        assert_eq!(image.width, 1);
        assert_eq!(image.height, 1);
        assert_eq!(image.pixels, vec![255, 0, 0, 255]);
        destroy_image(handle).expect("destroy");
        assert!(lookup_image(handle).is_none());
    }
}
