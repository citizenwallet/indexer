package common

import (
	"bytes"
	"image"
	"image/jpeg"
	"mime/multipart"

	"golang.org/x/image/draw"
)

type SizedImages struct {
	Big    []byte
	Medium []byte
	Small  []byte
}

func resizeImage(img image.Image, width, height int) image.Image {
	// Create a new image with the desired dimensions
	newImage := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw the original image onto the new one with interpolation to resize it
	draw.ApproxBiLinear.Scale(newImage, newImage.Bounds(), img, img.Bounds(), draw.Over, nil)

	return newImage
}

func imageToBytes(img image.Image) ([]byte, error) {
	// Create a buffer to hold the encoded image
	var buf bytes.Buffer

	// Encode the image as JPG and save it to the buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func ParseImage(file multipart.File) (*SizedImages, error) {
	// Parse the image data
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	si := &SizedImages{}

	// Resize the image to the big size
	big := resizeImage(img, 512, 512)
	si.Big, err = imageToBytes(big)
	if err != nil {
		return nil, err
	}

	// Resize the image to the medium size
	medium := resizeImage(img, 256, 256)
	si.Medium, err = imageToBytes(medium)
	if err != nil {
		return nil, err
	}

	// Resize the image to the small size
	small := resizeImage(img, 128, 128)
	si.Small, err = imageToBytes(small)
	if err != nil {
		return nil, err
	}

	return si, nil
}
