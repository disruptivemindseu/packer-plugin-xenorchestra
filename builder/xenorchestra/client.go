package xenorchestra

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

type VM struct {
	ID         string            `json:"id"`
	PowerState string            `json:"power_state"`
	Addresses  map[string]string `json:"addresses"`
}

type VDI struct {
	ID        string `json:"id"`
	NameLabel string `json:"name_label"`
	Size      int64  `json:"size"`
	SR        string `json:"$SR"`
}

type SR struct {
	ID        string `json:"id"`
	NameLabel string `json:"name_label"`
	Type      string `json:"SR_type"`
}

type Pool struct {
	ID        string `json:"id"`
	NameLabel string `json:"name_label"`
}

type Template struct {
	ID        string `json:"id"`
	NameLabel string `json:"name_label"`
}

type Network struct {
	ID        string `json:"id"`
	NameLabel string `json:"name_label"`
}

func NewClient(endpoint, token string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	jar.SetCookies(endpointURL, []*http.Cookie{
		{
			Name:  "authenticationToken",
			Value: token,
		},
	})

	return &Client{
		endpoint: endpoint,
		token:    token,
		http: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	url := c.endpoint + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) GetVM(vmID string) (*VM, error) {
	body, err := c.do("GET", fmt.Sprintf("/rest/v0/vms/%s", vmID), nil)
	if err != nil {
		return nil, err
	}

	var vm VM
	if err := json.Unmarshal(body, &vm); err != nil {
		return nil, err
	}

	return &vm, nil
}

func (c *Client) CreateVM(poolID string, config map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/rest/v0/pools/%s/actions/create_vm?sync=true", poolID)
	body, err := c.do("POST", path, config)
	if err != nil {
		return "", err
	}

	// Try to parse as {id, taskId}
	var result struct {
		ID     string `json:"id"`
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Return id if available, otherwise taskId
	if result.ID != "" {
		return result.ID, nil
	}
	return result.TaskID, nil
}

func (c *Client) StartVM(vmID string) error {
	_, err := c.do("POST", fmt.Sprintf("/vms/%s/actions/start", vmID), nil)
	return err
}

func (c *Client) ShutdownVM(vmID string) error {
	_, err := c.do("POST", fmt.Sprintf("/vms/%s/actions/clean_shutdown", vmID), nil)
	return err
}

func (c *Client) HardShutdownVM(vmID string) error {
	_, err := c.do("POST", fmt.Sprintf("/vms/%s/actions/hard_shutdown", vmID), nil)
	return err
}

func (c *Client) GetVDIs() ([]VDI, error) {
	body, err := c.do("GET", "/rest/v0/vdis", nil)
	if err != nil {
		return nil, err
	}

	// API returns array of VDI URIs like ["/rest/v0/vdis/uuid"]
	var vdiURIs []string
	if err := json.Unmarshal(body, &vdiURIs); err == nil {
		var vdis []VDI
		for _, uri := range vdiURIs {
			vdi, err := c.GetVDI(uri)
			if err != nil {
				continue
			}
			if vdi != nil {
				vdis = append(vdis, *vdi)
			}
		}
		return vdis, nil
	}

	// Fallback: try as VDI objects array
	var vdis []VDI
	if err := json.Unmarshal(body, &vdis); err == nil {
		return vdis, nil
	}

	// Fallback: try as map
	var vdisMap map[string]VDI
	if err := json.Unmarshal(body, &vdisMap); err == nil {
		result := make([]VDI, 0, len(vdisMap))
		for _, vdi := range vdisMap {
			result = append(result, vdi)
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to unmarshal VDIs response: %s", string(body))
}

func (c *Client) GetVDI(uri string) (*VDI, error) {
	body, err := c.do("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	var vdi VDI
	if err := json.Unmarshal(body, &vdi); err != nil {
		return nil, err
	}

	return &vdi, nil
}

func (c *Client) GetSRs() ([]SR, error) {
	body, err := c.do("GET", "/rest/v0/srs", nil)
	if err != nil {
		return nil, err
	}

	// API returns array of SR URIs like ["/rest/v0/srs/uuid"]
	var srURIs []string
	if err := json.Unmarshal(body, &srURIs); err == nil {
		var srs []SR
		for _, uri := range srURIs {
			sr, err := c.GetSR(uri)
			if err != nil {
				// Log but continue - some SRs might not be accessible
				continue
			}
			if sr != nil {
				srs = append(srs, *sr)
			}
		}
		return srs, nil
	}

	// Fallback: try as SR objects array
	var srs []SR
	if err := json.Unmarshal(body, &srs); err == nil {
		return srs, nil
	}

	// Fallback: try as map
	var srsMap map[string]SR
	if err := json.Unmarshal(body, &srsMap); err == nil {
		result := make([]SR, 0, len(srsMap))
		for _, sr := range srsMap {
			result = append(result, sr)
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to unmarshal SRs response: %s", string(body))
}

func (c *Client) GetSR(uri string) (*SR, error) {
	body, err := c.do("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	var sr SR
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, err
	}

	return &sr, nil
}

func (c *Client) GetPools() ([]Pool, error) {
	body, err := c.do("GET", "/rest/v0/pools", nil)
	if err != nil {
		return nil, err
	}

	// API returns array of Pool URIs like ["/rest/v0/pools/uuid"]
	var poolURIs []string
	if err := json.Unmarshal(body, &poolURIs); err == nil {
		var pools []Pool
		for _, uri := range poolURIs {
			pool, err := c.GetPool(uri)
			if err != nil {
				continue
			}
			if pool != nil {
				pools = append(pools, *pool)
			}
		}
		return pools, nil
	}

	// Fallback: try as Pool objects array
	var pools []Pool
	if err := json.Unmarshal(body, &pools); err == nil {
		return pools, nil
	}

	// Fallback: try as map
	var poolsMap map[string]Pool
	if err := json.Unmarshal(body, &poolsMap); err == nil {
		result := make([]Pool, 0, len(poolsMap))
		for _, pool := range poolsMap {
			result = append(result, pool)
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to unmarshal pools response: %s", string(body))
}

func (c *Client) GetPool(uri string) (*Pool, error) {
	body, err := c.do("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	var pool Pool
	if err := json.Unmarshal(body, &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

func (c *Client) GetTemplates() ([]Template, error) {
	body, err := c.do("GET", "/rest/v0/vm-templates", nil)
	if err != nil {
		return nil, err
	}

	// API returns array of Template URIs like ["/rest/v0/vm-templates/uuid"]
	var templateURIs []string
	if err := json.Unmarshal(body, &templateURIs); err == nil {
		var templates []Template
		for _, uri := range templateURIs {
			tmpl, err := c.GetTemplate(uri)
			if err != nil {
				continue
			}
			if tmpl != nil {
				templates = append(templates, *tmpl)
			}
		}
		return templates, nil
	}

	// Fallback: try as Template objects array
	var templates []Template
	if err := json.Unmarshal(body, &templates); err == nil {
		return templates, nil
	}

	// Fallback: try as map
	var templatesMap map[string]Template
	if err := json.Unmarshal(body, &templatesMap); err == nil {
		result := make([]Template, 0, len(templatesMap))
		for _, tmpl := range templatesMap {
			result = append(result, tmpl)
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to unmarshal templates response: %s", string(body))
}

func (c *Client) GetTemplate(uri string) (*Template, error) {
	body, err := c.do("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	var tmpl Template
	if err := json.Unmarshal(body, &tmpl); err != nil {
		return nil, err
	}

	return &tmpl, nil
}

func (c *Client) GetNetworks() ([]Network, error) {
	body, err := c.do("GET", "/rest/v0/networks", nil)
	if err != nil {
		return nil, err
	}

	// API returns array of Network URIs like ["/rest/v0/networks/uuid"]
	var networkURIs []string
	if err := json.Unmarshal(body, &networkURIs); err == nil {
		var networks []Network
		for _, uri := range networkURIs {
			network, err := c.GetNetwork(uri)
			if err != nil {
				continue
			}
			if network != nil {
				networks = append(networks, *network)
			}
		}
		return networks, nil
	}

	// Fallback: try as Network objects array
	var networks []Network
	if err := json.Unmarshal(body, &networks); err == nil {
		return networks, nil
	}

	// Fallback: try as map
	var networksMap map[string]Network
	if err := json.Unmarshal(body, &networksMap); err == nil {
		result := make([]Network, 0, len(networksMap))
		for _, network := range networksMap {
			result = append(result, network)
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to unmarshal networks response: %s", string(body))
}

func (c *Client) GetNetwork(uri string) (*Network, error) {
	body, err := c.do("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	var network Network
	if err := json.Unmarshal(body, &network); err != nil {
		return nil, err
	}

	return &network, nil
}
