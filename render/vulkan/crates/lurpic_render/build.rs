// Build script for lurpic_render.
//
// Two responsibilities:
//   1. Compile the GLSL shaders in src/shaders/ to SPIR-V with glslc, writing
//      them to OUT_DIR for `include_bytes!`. Checked-in .spv artifacts keep
//      offline CI working when glslc is absent.
//   2. Serialize the FFI symbol inventory (src/ffi_inventory.rs) to
//      OUT_DIR/ffi_inventory.json, the single source of truth the Go-side
//      codegen drift gate (`ffi_gen_test.go`) regenerates ffi_linux.c from.
use std::path::{Path, PathBuf};

fn main() {
    let manifest_dir = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR").unwrap());
    compile_shaders(&manifest_dir);
    export_ffi_inventory(&manifest_dir);
}

fn shader_entries(src: &Path) -> Vec<(PathBuf, String, String)> {
    // (path, stage, out_name)
    let mut out = Vec::new();
    let Ok(entries) = std::fs::read_dir(src) else {
        return out;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let Some(ext) = path.extension().and_then(|e| e.to_str()) else {
            continue;
        };
        let Some(stem) = path.file_stem().and_then(|s| s.to_str()).map(str::to_owned) else {
            continue;
        };
        match ext {
            "vert" => out.push((path, "vert".into(), format!("{}.vert.spv", stem))),
            "frag" => out.push((path, "frag".into(), format!("{}.frag.spv", stem))),
            "comp" => out.push((path, "comp".into(), format!("{}.comp.spv", stem))),
            _ => {}
        }
    }
    out
}

fn compile_shaders(manifest_dir: &Path) {
    let src = manifest_dir.join("src/shaders");
    let out_dir = std::env::var("OUT_DIR").unwrap();

    println!("cargo:rerun-if-changed=build.rs");
    for (path, _, _) in shader_entries(&src) {
        println!("cargo:rerun-if-changed={}", path.display());
    }
    // Track shared GLSL includes (e.g. coverage.glsl) so a shader edit that
    // only touches an include recompiles the dependents.
    if let Ok(entries) = std::fs::read_dir(&src) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.extension().and_then(|e| e.to_str()) == Some("glsl") {
                println!("cargo:rerun-if-changed={}", path.display());
            }
        }
    }

    // Baseline: copy the checked-in SPIR-V so `include_bytes!` works even when
    // glslc is unavailable (offline CI). glslc, when present, overwrites it.
    for (path, _, out_name) in shader_entries(&src) {
        // The checked-in artifact is the shader path + ".spv" (e.g.
        // solid.vert.spv).
        let spv = PathBuf::from(format!("{}.spv", path.display()));
        if spv.exists() {
            let out = Path::new(&out_dir).join(&out_name);
            std::fs::copy(&spv, &out).expect("copy checked-in SPIR-V");
        }
    }

    // Resolve glslc: LURPIC_GLSLC override, then VULKAN_SDK/bin/glslc, then PATH.
    let glslc = std::env::var("LURPIC_GLSLC").ok().or_else(|| {
        std::env::var("VULKAN_SDK").ok().and_then(|sdk| {
            let candidate = Path::new(&sdk).join("bin/glslc");
            candidate.exists().then(|| candidate.display().to_string())
        })
    }).unwrap_or_else(|| "glslc".to_string());

    let mut compiled_any = false;

    for (path, stage, out_name) in shader_entries(&src) {
        let output = Path::new(&out_dir).join(&out_name);
        let status = std::process::Command::new(&glslc)
            .arg(format!("-fshader-stage={}", stage))
            .arg(&path)
            .arg("-o")
            .arg(&output)
            .status();
        if matches!(status, Ok(s) if s.success()) {
            compiled_any = true;
        } else {
            eprintln!(
                "lurpic_render build.rs: glslc failed for {}; using checked-in SPIR-V",
                path.display()
            );
        }
    }
    if !compiled_any {
        eprintln!(
            "lurpic_render build.rs: install glslc (Vulkan SDK) or set LURPIC_GLSLC \
             to rebuild shaders; checked-in .spv artifacts are used otherwise."
        );
    }
}

fn export_ffi_inventory(manifest_dir: &Path) {
    let inventory_path = manifest_dir.join("src/ffi_inventory.rs");
    println!("cargo:rerun-if-changed={}", inventory_path.display());
    let source = std::fs::read_to_string(&inventory_path).unwrap_or_default();
    let out_dir = std::env::var("OUT_DIR").unwrap();
    let out = Path::new(&out_dir).join("ffi_inventory.json");
    std::fs::write(&out, extract_inventory(&source)).expect("write ffi_inventory.json");
}

/// Extracts `{ name, ret, args }` triples from the fixed inventory entry format:
///
/// ```text
/// FfiSymbol { name: "lurpic_render_version", ret: "const char *", args: "(void)" },
/// ```
fn extract_inventory(source: &str) -> String {
    let mut entries = Vec::new();
    let mut rest = source;
    while let Some(rel) = rest.find("name: \"") {
        rest = &rest[rel + "name: \"".len()..];
        let Some(name_end) = rest.find('"') else { break };
        let name = &rest[..name_end];
        rest = &rest[name_end..];

        let Some(ret_rel) = rest.find("ret: \"") else { break };
        rest = &rest[ret_rel + "ret: \"".len()..];
        let Some(ret_end) = rest.find('"') else { break };
        let ret = &rest[..ret_end];
        rest = &rest[ret_end..];

        let Some(args_rel) = rest.find("args: \"") else { break };
        rest = &rest[args_rel + "args: \"".len()..];
        let Some(args_end) = rest.find('"') else { break };
        let args = &rest[..args_end];
        rest = &rest[args_end..];

        entries.push(format!(
            "{{\"name\":\"{}\",\"ret\":\"{}\",\"args\":\"{}\"}}",
            name, ret, args
        ));
    }
    format!("[{}]", entries.join(",\n"))
}
