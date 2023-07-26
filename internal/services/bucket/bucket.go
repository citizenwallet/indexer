package bucket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
)

const (
	PinFileURL = "/pinning/pinFileToIPFS"
	PinJSONURL = "/pinning/pinJSONToIPFS"
	UnpinURL   = "/pinning/unpin"
)

type PinResponse struct {
	IpfsHash  string `json:"IpfsHash"`
	PinSize   int    `json:"PinSize"`
	Timestamp string `json:"Timestamp"`
}

type Bucket struct {
	BaseURL   string
	APIKey    string
	APISecret string
}

func NewBucket(baseURL, apiKey, apiSecret string) *Bucket {
	return &Bucket{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
	}
}

func (b *Bucket) PinJSONToIPFS(ctx context.Context, data []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+PinJSONURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("pinata_api_key", b.APIKey)
	req.Header.Add("pinata_secret_api_key", b.APISecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	var pinResp PinResponse
	if err := json.NewDecoder(resp.Body).Decode(&pinResp); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("error pinning to ipfs")
	}

	return pinResp.IpfsHash, nil
}

func (b *Bucket) PinFileToIPFS(ctx context.Context, file []byte, name string) (string, error) {
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	part1, err := writer.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}

	_, err = part1.Write(file)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+PinFileURL, payload)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("pinata_api_key", b.APIKey)
	req.Header.Add("pinata_secret_api_key", b.APISecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var pinResp PinResponse
	if err := json.NewDecoder(resp.Body).Decode(&pinResp); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("error unpinning from ipfs")
	}

	return fmt.Sprintf("ipfs://%s", pinResp.IpfsHash), nil
}

func (b *Bucket) Unpin(ctx context.Context, hash string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, b.BaseURL+UnpinURL+"/"+hash, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("pinata_api_key", b.APIKey)
	req.Header.Add("pinata_secret_api_key", b.APISecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("error unpinning from ipfs")
	}

	return nil
}
