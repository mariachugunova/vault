package vault

import (
	"strings"

	"github.com/hashicorp/vault/logical"
)

// SystemBackend implements logical.Backend and is used to interact with
// the core of the system. This backend is hardcoded to exist at the "sys"
// prefix. Conceptually it is similar to procfs on Linux.
type SystemBackend2 struct {
	Core *Core
}

func (b *SystemBackend2) HandleRequest(req *logical.Request) (*logical.Response, error) {
	// Switch on the path to route to the appropriate handler
	switch {
	case req.Path == "mounts":
		return b.handleMountTable(req)
	case strings.HasPrefix(req.Path, "mount/"):
		return b.handleMountOperation(req)
	case req.Path == "remount":
		return b.handleRemount(req)
	default:
		return nil, ErrUnsupportedPath
	}
}

func (b *SystemBackend2) RootPaths() []string {
	return []string{
		"mount/*",
		"remount",
	}
}

// handleMountTable handles the "mounts" endpoint to provide the mount table
func (b *SystemBackend2) handleMountTable(req *logical.Request) (*logical.Response, error) {
	switch req.Operation {
	case logical.ReadOperation:
	default:
		return nil, ErrUnsupportedOperation
	}

	b.Core.mountsLock.RLock()
	defer b.Core.mountsLock.RUnlock()

	resp := &logical.Response{
		IsSecret: false,
		Data:     make(map[string]interface{}),
	}
	for _, entry := range b.Core.mounts.Entries {
		info := map[string]string{
			"type":        entry.Type,
			"description": entry.Description,
		}
		resp.Data[entry.Path] = info
	}

	return resp, nil
}

// handleMountOperation is used to mount or unmount a path
func (b *SystemBackend2) handleMountOperation(req *logical.Request) (*logical.Response, error) {
	switch req.Operation {
	case logical.WriteOperation:
		return b.handleMount(req)
	case logical.DeleteOperation:
		return b.handleUnmount(req)
	default:
		return nil, ErrUnsupportedOperation
	}
}

// handleMount is used to mount a new path
func (b *SystemBackend2) handleMount(req *logical.Request) (*logical.Response, error) {
	suffix := strings.TrimPrefix(req.Path, "mount/")
	if len(suffix) == 0 {
		return logical.ErrorResponse("path cannot be blank"), ErrInvalidRequest
	}

	// Get the type and description (optionally)
	logicalType := req.GetString("type")
	if logicalType == "" {
		return logical.ErrorResponse("backend type must be specified as a string"), ErrInvalidRequest
	}
	description := req.GetString("description")

	// Create the mount entry
	me := &MountEntry{
		Path:        suffix,
		Type:        logicalType,
		Description: description,
	}

	// Attempt mount
	if err := b.Core.mount(me); err != nil {
		return logical.ErrorResponse(err.Error()), ErrInvalidRequest
	}
	return nil, nil
}

// handleUnmount is used to unmount a path
func (b *SystemBackend2) handleUnmount(req *logical.Request) (*logical.Response, error) {
	suffix := strings.TrimPrefix(req.Path, "mount/")
	if len(suffix) == 0 {
		return logical.ErrorResponse("path cannot be blank"), ErrInvalidRequest
	}

	// Attempt unmount
	if err := b.Core.unmount(suffix); err != nil {
		return logical.ErrorResponse(err.Error()), ErrInvalidRequest
	}

	return nil, nil
}

// handleRemount is used to remount a path
func (b *SystemBackend2) handleRemount(req *logical.Request) (*logical.Response, error) {
	// Only accept write operations
	switch req.Operation {
	case WriteOperation:
	default:
		return nil, ErrUnsupportedOperation
	}

	// Get the paths
	fromPath := req.GetString("from")
	toPath := req.GetString("to")
	if fromPath == "" || toPath == "" {
		return logical.ErrorResponse(
				"both 'from' and 'to' path must be specified as a string"),
			ErrInvalidRequest
	}

	// Attempt remount
	if err := b.Core.remount(fromPath, toPath); err != nil {
		return logical.ErrorResponse(err.Error()), ErrInvalidRequest
	}

	return nil, nil
}
