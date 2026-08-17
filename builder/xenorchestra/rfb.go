package xenorchestra

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// RFBClient handles RFB protocol over WebSocket
type RFBClient struct {
	ws *websocket.Conn
}

// KeySym X11 keysyms for special keys
var KeySym = map[string]uint32{
	"Enter":     0xff0d,
	"Tab":       0xff09,
	"Escape":    0xff1b,
	"Backspace": 0xff08,
	"Delete":    0xffff,
	"Up":        0xff52,
	"Down":      0xff54,
	"Left":      0xff51,
	"Right":     0xff53,
	"Home":      0xff50,
	"End":       0xff57,
	"Control":   0xffe3,
	"Alt":       0xffe9,
	"Shift":     0xffe1,
}

// NewRFBClient creates and initializes an RFB client
func NewRFBClient(ws *websocket.Conn) (*RFBClient, error) {
	client := &RFBClient{ws: ws}

	// Read initial RFB handshake from server
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, initialData, err := ws.ReadMessage()
	ws.SetReadDeadline(time.Time{})

	if err != nil {
		return nil, fmt.Errorf("failed to read RFB version: %w", err)
	}

	if len(initialData) < 12 || string(initialData[:3]) != "RFB" {
		return nil, fmt.Errorf("invalid RFB version: %s", string(initialData))
	}

	// Send RFB version back
	versionMsg := []byte("RFB 003.008\n")
	if err := ws.WriteMessage(websocket.BinaryMessage, versionMsg); err != nil {
		return nil, fmt.Errorf("failed to send RFB version: %w", err)
	}

	// Complete handshake: read security types, send selection, read auth result, send ClientInit
	for i := 0; i < 5; i++ {
		ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, data, readErr := ws.ReadMessage()
		ws.SetReadDeadline(time.Time{})

		if readErr != nil {
			break
		}

		// For security types, select the first one
		if i == 0 && len(data) >= 2 {
			selectedType := data[1]
			ws.WriteMessage(websocket.BinaryMessage, []byte{selectedType})
		}

		// For auth response (4 bytes), send ClientInit
		if len(data) == 4 {
			status := binary.BigEndian.Uint32(data)
			if status != 0 {
				return nil, fmt.Errorf("RFB authentication failed: status %d", status)
			}
			ws.WriteMessage(websocket.BinaryMessage, []byte{1})
			break
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Send FramebufferUpdateRequest to start getting updates (we'll ignore them)
	fbReq := make([]byte, 10)
	fbReq[0] = 3 // FramebufferUpdateRequest
	ws.WriteMessage(websocket.BinaryMessage, fbReq)

	// Start goroutine to drain incoming messages (we don't need framebuffer updates)
	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	return client, nil
}

// SendKey sends an RFB key event
func (c *RFBClient) SendKey(keysym uint32, down bool) error {
	buf := make([]byte, 8)
	buf[0] = 4 // Key event message type
	if down {
		buf[1] = 1
	} else {
		buf[1] = 0
	}
	binary.BigEndian.PutUint32(buf[4:], keysym)
	return c.ws.WriteMessage(websocket.BinaryMessage, buf)
}

// SendText sends text to the console with proper shift handling
func (c *RFBClient) SendText(text string) error {
	for _, ch := range text {
		keysym, needsShift := charToKeysym(ch)

		// Send shift if needed
		if needsShift {
			if err := c.SendKey(KeySym["Shift"], true); err != nil {
				return err
			}
			time.Sleep(20 * time.Millisecond)
		}

		if err := c.SendKey(keysym, true); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
		if err := c.SendKey(keysym, false); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)

		// Release shift if needed
		if needsShift {
			if err := c.SendKey(KeySym["Shift"], false); err != nil {
				return err
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

// SendEnter sends the Enter key
func (c *RFBClient) SendEnter() error {
	if err := c.SendKey(KeySym["Enter"], true); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return c.SendKey(KeySym["Enter"], false)
}

// SendCommand sends a boot command with special key support
// Syntax: regular text mixed with <keyname> for special keys
// Examples: "text<enter>", "ctrl<alt><del>"
func (c *RFBClient) SendCommand(command string) error {
	i := 0
	for i < len(command) {
		// Check for special key syntax <...>
		if command[i] == '<' {
			closeIdx := strings.Index(command[i:], ">")
			if closeIdx == -1 {
				// No closing >, treat as literal
				if err := c.SendText("<"); err != nil {
					return err
				}
				i++
				continue
			}

			keyName := command[i+1 : i+closeIdx]
			// Capitalize first letter for key lookup (e.g., "enter" -> "Enter")
			capitalizedKey := strings.ToUpper(keyName[:1]) + keyName[1:]
			keysym, ok := KeySym[capitalizedKey]
			if !ok {
				// Unknown key, send literally
				if err := c.SendText("<" + keyName + ">"); err != nil {
					return err
				}
				i += closeIdx + 1
				continue
			}

			// Send the special key
			if err := c.SendKey(keysym, true); err != nil {
				return err
			}
			time.Sleep(50 * time.Millisecond)
			if err := c.SendKey(keysym, false); err != nil {
				return err
			}
			time.Sleep(50 * time.Millisecond)

			i += closeIdx + 1
		} else {
			// Regular character - accumulate text until next < or end
			textEnd := i
			for textEnd < len(command) && command[textEnd] != '<' {
				textEnd++
			}

			text := command[i:textEnd]
			if err := c.SendText(text); err != nil {
				return err
			}
			i = textEnd
		}
	}
	return nil
}

func charToKeysym(ch rune) (uint32, bool) {
	// Characters that require shift modifier (shift+key = result)
	shiftChars := map[rune]uint32{
		'!': 0x31, // Shift+1
		'@': 0x32, // Shift+2
		'#': 0x33, // Shift+3
		'$': 0x34, // Shift+4
		'%': 0x35, // Shift+5
		'^': 0x36, // Shift+6
		'&': 0x37, // Shift+7
		'*': 0x38, // Shift+8
		'(': 0x39, // Shift+9
		')': 0x30, // Shift+0
		'_': 0x2d, // Shift+-
		'+': 0x3d, // Shift+=
		'{': 0x5b, // Shift+[
		'}': 0x5d, // Shift+]
		'|': 0x5c, // Shift+\
		':': 0x3b, // Shift+;
		'"': 0x27, // Shift+'
		'<': 0x2c, // Shift+,
		'>': 0x2e, // Shift+.
		'?': 0x2f, // Shift+/
		'~': 0x60, // Shift+`
	}

	if keysym, hasShift := shiftChars[ch]; hasShift {
		return keysym, true
	}

	// Regular characters (0-9, a-z, etc.)
	cp := uint32(ch)
	if cp < 0x100 {
		return cp, false
	}
	return 0x01000000 + cp, false
}

// ConnectWithAuth handles signin and WebSocket connection
func ConnectWithAuth(endpoint, username, password, vmUUID string) (*websocket.Conn, error) {
	// Parse endpoint to derive scheme and host
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host := parsedURL.Host
	if host == "" {
		host = parsedURL.Hostname()
	}

	// Determine HTTP and WebSocket schemes based on endpoint
	httpScheme := "https"
	wsScheme := "wss"
	if scheme == "http" {
		httpScheme = "http"
		wsScheme = "ws"
	}

	// Create HTTP client with cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	httpClient := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// Sign in to get session cookies
	signinURL := fmt.Sprintf("%s://%s/signin/local", httpScheme, host)
	body := url.Values{
		"username": {username},
		"password": {password},
	}.Encode()

	resp, err := httpClient.Post(
		signinURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("signin failed: %w", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("signin failed with status %d", resp.StatusCode)
	}

	// Connect to WebSocket with cookies from jar
	wsURL := fmt.Sprintf("%s://%s/api/consoles/%s", wsScheme, host, vmUUID)
	httpURL := fmt.Sprintf("%s://%s/api/consoles/%s", httpScheme, host, vmUUID)
	u, _ := url.Parse(httpURL)

	header := http.Header{}
	for _, cookie := range jar.Cookies(u) {
		header.Add("Cookie", cookie.String())
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	ws, _, err := dialer.DialContext(context.Background(), wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	return ws, nil
}
