package xenorchestra

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/config"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
	v2 "github.com/vatesfr/xenorchestra-go-sdk/v2"
	v2client "github.com/vatesfr/xenorchestra-go-sdk/v2/client"
)

// Client wraps the xenorchestra-go-sdk v2 (REST) library. rest is the SDK's raw REST
// client, used for what the library doesn't cover yet (templates, boot firmware).
type Client struct {
	library.Library
	rest *v2client.Client
}

func NewClient(endpoint, token string) (*Client, error) {
	cfg := &config.Config{Url: endpoint, Token: token}
	xo, err := v2.New(cfg)
	if err != nil {
		return nil, err
	}
	rest, err := v2client.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Library: xo, rest: rest}, nil
}

// byName builds an XO filter; it may match more than the exact name, callers compare NameLabel.
func byName(name string) string {
	return fmt.Sprintf("name_label:%q", name)
}

// FindTemplate returns the UUID of the VM template with the given name label.
func (c *Client) FindTemplate(ctx context.Context, name string) (uuid.UUID, error) {
	var templates []struct {
		ID        string `json:"id"`
		NameLabel string `json:"name_label"`
	}
	params := map[string]any{"fields": "id,name_label", "filter": byName(name)}
	if err := v2client.TypedGet(ctx, c.rest, "vm-templates", params, &templates); err != nil {
		return uuid.Nil, err
	}
	for _, t := range templates {
		if t.NameLabel != name {
			continue
		}
		// Built-in template IDs are "<pool uuid>-<template uuid>"; the API wants the last part.
		id := t.ID
		if parts := strings.Split(id, "-"); len(parts) >= 10 {
			id = strings.Join(parts[5:], "-")
		}
		return uuid.FromString(id)
	}
	return uuid.Nil, fmt.Errorf("template '%s' not found", name)
}

// CreateVM creates a VM and waits for it. Same as Pool().CreateVM, plus cpus and
// hvmBootFirmware which payloads.CreateVMParams doesn't expose.
func (c *Client) CreateVM(ctx context.Context, poolID uuid.UUID, params payloads.CreateVMParams, cpus int, firmware string) (uuid.UUID, error) {
	body := struct {
		payloads.CreateVMParams
		CPUs            int    `json:"cpus,omitempty"`
		HVMBootFirmware string `json:"hvmBootFirmware,omitempty"`
	}{params, cpus, firmware}

	var resp payloads.TaskIDResponse
	if err := v2client.TypedPost(ctx, c.rest, fmt.Sprintf("pools/%s/actions/create_vm", poolID), body, &resp); err != nil {
		return uuid.Nil, err
	}
	task, err := c.Task().HandleTaskResponse(ctx, resp, true)
	if err != nil {
		return uuid.Nil, err
	}
	if task.Status != payloads.Success {
		return uuid.Nil, fmt.Errorf("VM creation failed: %s", task.Result.Message)
	}
	return task.Result.ID, nil
}
