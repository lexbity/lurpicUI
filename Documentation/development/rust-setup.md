# Rust Setup

The Vulkan backend in `render/vulkan` depends on the Rust toolchain for the
bridge crate in `render/vulkan/crates/lurpic_render/`.

## Install

Use `rustup` on Linux:

```sh
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

After installation, ensure these commands are available on `PATH`:

```sh
cargo --version
rustc --version
```

## Build the bridge crate

From the repository root:

```sh
cargo build --manifest-path render/vulkan/crates/lurpic_render/Cargo.toml
cargo test --manifest-path render/vulkan/crates/lurpic_render/Cargo.toml
```

The Go tests in `render/vulkan` will also build the crate automatically before
calling the Rust `lurpic_render_version()` symbol.

## Optional override

If you want `render/vulkan` to load a specific shared library, set
`LURPIC_RENDER_RUST_LIBRARY` to the full path of `liblurpic_render.so`.

To override which Vulkan loader the Rust bridge uses, set
`LURPIC_RENDER_VULKAN_LIBRARY` to a specific `libvulkan.so` path. If unset,
the bridge tries the usual system loader names.
