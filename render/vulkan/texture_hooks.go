package vulkan

import (
	"runtime"

	"codeburg.org/lexbit/lurpicui/render"
)

func init() {
	render.RegisterVulkanTextureHooks(render.VulkanTextureHooks{
		Upload: func(req render.TextureUploadRequest) (render.TextureID, error) {
			id, err := UploadImage(req.PixelData, int(req.Width), int(req.Height), int(req.Width)*4, uint32(render.TextureFormatRGBA8))
			return render.TextureID(id), err
		},
		Free: func(id render.TextureID) {
			_ = DestroyImage(uint64(id))
		},
		Budget: func(target render.TextureFormat) int {
			if target == render.TextureFormatASTC4x4 || runtime.GOOS == "android" {
				return 4 << 20
			}
			return 16 << 20
		},
		Target: func() render.TextureFormat {
			if runtime.GOOS == "android" {
				return render.TextureFormatASTC4x4
			}
			return render.TextureFormatBC7
		},
	})
}
