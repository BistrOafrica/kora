// Package graph owns resource identity resolution and canonical hashing
// (GRAPH-001). It is deliberately dependency-light: it imports only the
// contract package, never orm, api, or configstore, so it can be used by every
// layer that must address a resource identically.
package graph

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/asenawritescode/kora/contract"
)

// CanonicalHash returns the SHA-256 hex digest of v marshalled to canonical
// JSON. Map keys are sorted recursively, struct fields follow declared order,
// and slices keep element order. The result is therefore stable across
// equivalent inputs that differ only in map key ordering.
func CanonicalHash(v any) string {
	raw, err := canonicalMarshal(v)
	if err != nil {
		// An un-marshalable value is a programming error; fall back to the
		// default encoder so the digest is at least deterministic per process.
		raw, _ = json.Marshal(v)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// DescriptorHash returns the canonical hash of a resource descriptor's content
// — namespace, name, version, kind, fields, and dependencies — excluding the
// Hash field and lifecycle timestamps. This is the value stored on
// ResourceDescriptor.Hash.
func DescriptorHash(d contract.ResourceDescriptor) string {
	content := descriptorContent{
		Namespace: d.Ref.Namespace,
		Name:      d.Ref.Name,
		Version:   d.Ref.Version,
		Kind:      d.Kind,
		Fields:    d.Fields,
		DependsOn: d.DependsOn,
	}
	return CanonicalHash(content)
}

// descriptorContent is the stable, hash-relevant projection of a descriptor.
// Field order is fixed so json.Marshal is deterministic without relying on map
// iteration order.
type descriptorContent struct {
	Namespace string                 `json:"namespace"`
	Name      string                 `json:"name"`
	Version   int                    `json:"version"`
	Kind      contract.ResourceKind  `json:"kind"`
	Fields    []contract.TypedField  `json:"fields,omitempty"`
	DependsOn []contract.ResourceRef `json:"depends_on,omitempty"`
}

// canonicalMarshal encodes v with recursively sorted map keys. Structs,
// slices of concrete types, and primitives fall through to encoding/json.
func canonicalMarshal(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			vb, err := canonicalMarshal(t[k])
			if err != nil {
				return nil, err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			buf.Write(vb)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			eb, err := canonicalMarshal(e)
			if err != nil {
				return nil, err
			}
			buf.Write(eb)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		return json.Marshal(v)
	}
}
