use std::collections::{HashMap, VecDeque};
use std::sync::{Mutex, OnceLock};

#[derive(Clone, Debug)]
pub struct GlyphBitmap {
    pub width: u32,
    pub height: u32,
    pub pixels: Vec<u8>,
    pub offset_x: f32,
    pub offset_y: f32,
    #[allow(dead_code)]
    pub advance: f32,
}

#[derive(Clone, Debug)]
struct GlyphVariants {
    bitmap: GlyphBitmap,
    sdf: Option<GlyphBitmap>,
}

#[derive(Clone, Debug, PartialEq, Eq, Hash)]
struct GlyphKey {
    font_id: u64,
    glyph_id: u32,
    size_bits: u32,
}

#[derive(Default)]
struct GlyphAtlas {
    entries: HashMap<GlyphKey, GlyphVariants>,
    lru: VecDeque<GlyphKey>,
    evictions: usize,
    capacity: usize,
}

impl GlyphAtlas {
    fn new() -> Self {
        Self {
            entries: HashMap::new(),
            lru: VecDeque::new(),
            evictions: 0,
            capacity: 1024,
        }
    }

    fn upload(&mut self, font_id: u64, glyph_id: u32, size_bits: u32, bitmap: GlyphBitmap) {
        let key = GlyphKey {
            font_id,
            glyph_id,
            size_bits,
        };
        let variants = GlyphVariants {
            sdf: Some(generate_sdf(&bitmap)),
            bitmap,
        };
        if self.entries.contains_key(&key) {
            self.touch(&key);
            self.entries.insert(key, variants);
            return;
        }
        while self.entries.len() >= self.capacity {
            if let Some(oldest) = self.lru.pop_front() {
                if self.entries.remove(&oldest).is_some() {
                    self.evictions += 1;
                }
            } else {
                break;
            }
        }
        self.lru.push_back(key.clone());
        self.entries.insert(key, variants);
    }

    fn lookup(&mut self, font_id: u64, glyph_id: u32, size_bits: u32) -> Option<GlyphBitmap> {
        self.lookup_with_mode(font_id, glyph_id, size_bits, false)
    }

    fn lookup_with_mode(
        &mut self,
        font_id: u64,
        glyph_id: u32,
        size_bits: u32,
        use_sdf: bool,
    ) -> Option<GlyphBitmap> {
        let key = GlyphKey {
            font_id,
            glyph_id,
            size_bits,
        };
        let bitmap = self.entries.get(&key).map(|entry| {
            if use_sdf {
                entry.sdf.clone().unwrap_or_else(|| entry.bitmap.clone())
            } else {
                entry.bitmap.clone()
            }
        });
        if bitmap.is_some() {
            self.touch(&key);
        }
        bitmap
    }

    fn touch(&mut self, key: &GlyphKey) {
        if let Some(pos) = self.lru.iter().position(|k| k == key) {
            self.lru.remove(pos);
        }
        self.lru.push_back(key.clone());
    }
}

static ATLAS: OnceLock<Mutex<GlyphAtlas>> = OnceLock::new();

fn atlas() -> &'static Mutex<GlyphAtlas> {
    ATLAS.get_or_init(|| Mutex::new(GlyphAtlas::new()))
}

pub fn upload_glyph(font_id: u64, glyph_id: u32, size_bits: u32, bitmap: GlyphBitmap) {
    let mut atlas = atlas().lock().expect("glyph atlas mutex poisoned");
    atlas.upload(font_id, glyph_id, size_bits, bitmap);
}

pub fn lookup_glyph(font_id: u64, glyph_id: u32, size_bits: u32) -> Option<GlyphBitmap> {
    let mut atlas = atlas().lock().expect("glyph atlas mutex poisoned");
    atlas.lookup(font_id, glyph_id, size_bits)
}

pub fn lookup_glyph_sdf(font_id: u64, glyph_id: u32, size_bits: u32) -> Option<GlyphBitmap> {
    let mut atlas = atlas().lock().expect("glyph atlas mutex poisoned");
    atlas.lookup_with_mode(font_id, glyph_id, size_bits, true)
}

pub fn atlas_stats() -> (usize, usize) {
    let atlas = atlas().lock().expect("glyph atlas mutex poisoned");
    (atlas.entries.len(), atlas.evictions)
}

pub fn reset_atlas() {
    let mut atlas = atlas().lock().expect("glyph atlas mutex poisoned");
    *atlas = GlyphAtlas::new();
}

fn generate_sdf(bitmap: &GlyphBitmap) -> GlyphBitmap {
    if bitmap.width == 0 || bitmap.height == 0 || bitmap.pixels.is_empty() {
        return bitmap.clone();
    }
    let w = bitmap.width as usize;
    let h = bitmap.height as usize;
    let mut sdf = vec![0u8; w * h];
    let spread = ((w.max(h) as f32) * 0.35).max(1.0);
    for y in 0..h {
        for x in 0..w {
            let idx = y * w + x;
            let inside = bitmap.pixels[idx] > 127;
            let mut best = f32::INFINITY;
            for yy in 0..h {
                for xx in 0..w {
                    let other = bitmap.pixels[yy * w + xx] > 127;
                    if other == inside {
                        continue;
                    }
                    let dx = x as f32 - xx as f32;
                    let dy = y as f32 - yy as f32;
                    let d = (dx * dx + dy * dy).sqrt();
                    if d < best {
                        best = d;
                    }
                }
            }
            if !best.is_finite() {
                best = spread;
            }
            let signed = if inside { -best } else { best };
            let normalized = (0.5 - signed / (spread * 2.0)).clamp(0.0, 1.0);
            sdf[idx] = (normalized * 255.0) as u8;
        }
    }
    GlyphBitmap {
        width: bitmap.width,
        height: bitmap.height,
        pixels: sdf,
        offset_x: bitmap.offset_x,
        offset_y: bitmap.offset_y,
        advance: bitmap.advance,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn upload_and_lookup_glyph() {
        reset_atlas();
        upload_glyph(
            1,
            2,
            3,
            GlyphBitmap {
                width: 1,
                height: 1,
                pixels: vec![255],
                offset_x: 0.0,
                offset_y: 0.0,
                advance: 1.0,
            },
        );
        let got = lookup_glyph(1, 2, 3).expect("glyph present");
        assert_eq!(got.pixels, vec![255]);
    }

    #[test]
    fn evicts_when_full() {
        reset_atlas();
        let mut atlas = atlas().lock().expect("glyph atlas mutex poisoned");
        atlas.capacity = 1;
        drop(atlas);
        upload_glyph(
            1,
            1,
            1,
            GlyphBitmap {
                width: 1,
                height: 1,
                pixels: vec![1],
                offset_x: 0.0,
                offset_y: 0.0,
                advance: 1.0,
            },
        );
        upload_glyph(
            1,
            2,
            1,
            GlyphBitmap {
                width: 1,
                height: 1,
                pixels: vec![2],
                offset_x: 0.0,
                offset_y: 0.0,
                advance: 1.0,
            },
        );
        let (count, evictions) = atlas_stats();
        assert_eq!(count, 1);
        assert!(evictions >= 1);
    }
}
