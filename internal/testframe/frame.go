// Package testframe constructs explicitly budgeted immutable render fixtures.
// Production frame construction belongs to runtime owners and must use their
// configured wallet instead of this fixture-local wallet.
package testframe

import (
	"image"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
)

func Seal(input render.FrameInput) *render.Frame {
	bytes, err := render.EstimateFrameBytes(input)
	if err != nil {
		panic(err)
	}
	if bytes == 0 {
		bytes = 1
	}
	wallet, err := membudget.NewWallet(bytes)
	if err != nil {
		panic(err)
	}
	reservation, err := wallet.Reserve(bytes)
	if err != nil {
		panic(err)
	}
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		_ = reservation.Cancel()
		panic(err)
	}
	return frame
}

func Group(frames ...*render.Frame) *render.FrameGroup {
	group, err := render.TakeFrameGroup(frames)
	if err != nil {
		panic(err)
	}
	return group
}

func Image(value any) gfx.ImageHandle {
	if value == nil {
		return gfx.ImageHandle{}
	}
	if handle, ok := value.(gfx.ImageHandle); ok {
		return handle
	}
	src, ok := value.(*image.RGBA)
	if !ok {
		panic("testframe: unsupported image fixture type")
	}
	image, err := gfx.NewImageHandle(src, 1)
	if err != nil {
		panic(err)
	}
	return image
}
