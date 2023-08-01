package common

import (
	"bytes"
	"errors"
	"image"
	"image/gif"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"

	"golang.org/x/image/draw"
)

type ImageFormat string

const (
	JPG  ImageFormat = "jpg"
	JPEG ImageFormat = "jpeg"
	PNG  ImageFormat = "png"
	GIF  ImageFormat = "gif"
	WEBP ImageFormat = "webp"
)

type SizedImages struct {
	Big    []byte
	Medium []byte
	Small  []byte
}

func resizeImage(img image.Image, width int) image.Image {
	// Calculate the height based on the original aspect ratio
	aspectRatio := float64(img.Bounds().Dy()) / float64(img.Bounds().Dx())
	height := int(float64(width) * aspectRatio)

	// Create a new image with the desired dimensions
	newImage := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw the original image onto the new one with interpolation to resize it
	draw.ApproxBiLinear.Scale(newImage, newImage.Bounds(), img, img.Bounds(), draw.Over, nil)

	return newImage
}

func imageToBytes(img image.Image, format ImageFormat) ([]byte, error) {
	// Create a buffer to hold the encoded image
	var buf bytes.Buffer

	switch format {
	case JPG, JPEG:
		err := jpeg.Encode(&buf, img, nil)
		if err != nil {
			return nil, err
		}
	case PNG:
		err := png.Encode(&buf, img)
		if err != nil {
			return nil, err
		}
	case GIF:
		err := gif.Encode(&buf, img, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported image format: " + string(format))
	}

	return buf.Bytes(), nil
}

func ParseImage(file multipart.File) (*SizedImages, error) {
	// Parse the image data
	img, f, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	si := &SizedImages{}

	// Resize the image to the big size
	big := resizeImage(img, 512)
	si.Big, err = imageToBytes(big, ImageFormat(f))
	if err != nil {
		return nil, err
	}

	// Resize the image to the medium size
	medium := resizeImage(img, 256)
	si.Medium, err = imageToBytes(medium, ImageFormat(f))
	if err != nil {
		return nil, err
	}

	// Resize the image to the small size
	small := resizeImage(img, 128)
	si.Small, err = imageToBytes(small, ImageFormat(f))
	if err != nil {
		return nil, err
	}

	return si, nil
}
