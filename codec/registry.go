package codec

import "sync"

// Registry manages the available codecs
type Registry struct {
	mu     sync.RWMutex
	codecs map[string]Codec // key can be either name or UID
}

var defaultRegistry = &Registry{
	codecs: make(map[string]Codec),
}

// Register registers a codec using both its name and UID
func Register(codec Codec) {
	defaultRegistry.Register(codec)
}

// Get retrieves a codec by name or UID
func Get(nameOrUID string) (Codec, error) {
	return defaultRegistry.Get(nameOrUID)
}

// List returns all registered codecs
func List() []Codec {
	return defaultRegistry.List()
}

// Register registers a codec using both its name and UID
func (r *Registry) Register(codec Codec) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Register by both name and UID
	r.codecs[codec.Name()] = codec
	r.codecs[codec.UID()] = codec
}

// Get retrieves a codec by name or UID
func (r *Registry) Get(nameOrUID string) (Codec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codec, ok := r.codecs[nameOrUID]
	if !ok {
		return nil, ErrCodecNotFound
	}
	return codec, nil
}

// List returns all registered codecs (deduplicated)
func (r *Registry) List() []Codec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[Codec]bool)
	codecs := make([]Codec, 0)

	for _, codec := range r.codecs {
		if !seen[codec] {
			seen[codec] = true
			codecs = append(codecs, codec)
		}
	}

	return codecs
}
