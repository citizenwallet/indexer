package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

type Message struct {
	Content string `json:"content"`
}

type Messager struct {
	BaseURL   string
	ChainName string

	notify bool
}

func NewMessager(baseURL, chainName string, notify bool) indexer.WebhookMessager {
	return &Messager{
		BaseURL:   baseURL,
		ChainName: chainName,
		notify:    notify,
	}
}

func (b *Messager) Notify(ctx context.Context, message string) error {
	if !b.notify {
		return nil
	}

	data, err := json.Marshal(Message{Content: fmt.Sprintf("[%s] %s", b.ChainName, message)})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("error sending message")
	}

	return nil
}

func (b *Messager) NotifyWarning(ctx context.Context, errorMessage error) error {
	if !b.notify {
		return nil
	}

	data, err := json.Marshal(Message{Content: fmt.Sprintf("[%s] warning: %s", b.ChainName, errorMessage.Error())})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("error sending message")
	}

	return nil
}

func (b *Messager) NotifyError(ctx context.Context, errorMessage error) error {
	if !b.notify {
		return nil
	}

	data, err := json.Marshal(Message{Content: fmt.Sprintf("[%s] error: %s", b.ChainName, errorMessage.Error())})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("error sending message")
	}

	return nil
}
