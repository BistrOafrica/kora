# Kora SDK for Go

Go types and interfaces for building script extensions, provider integrations, and tools for Kora.

## Installation

```bash
go get github.com/asenawritescode/kora/sdk@latest
```

## Usage

```go
import "github.com/asenawritescode/kora/sdk"
```

The SDK provides interfaces that scripts and extensions implement against:

### DocProvider — CRUD on Kora documents

```go
type DocProvider interface {
    GetDoc(doctype, name string) (map[string]any, error)
    GetList(doctype string, filters map[string]any, orderBy string, limit, offset int) ([]map[string]any, error)
    SaveDoc(doctype string, doc map[string]any, modifiedBy string) error
    CreateDoc(doctype string, doc map[string]any, owner, modifiedBy string) (map[string]any, error)
    DeleteDoc(doctype, name string) error
}
```

### SecretProvider — retrieve API keys and secrets

```go
type SecretProvider interface {
    GetSecret(key string) (string, error)
}
```

### HTTPProvider — make external HTTP requests from scripts

```go
type HTTPProvider interface {
    DoHTTP(req *HTTPRequest) (*HTTPResponse, error)
}
```

### KoraProvider — composed interface for script contexts

```go
type KoraProvider interface {
    DocProvider
    SecretProvider
    HTTPProvider
}
```

### Script support

Pre-defined constants for script types (`DocEvent`, `APIMethod`, `WorkflowAction`, `Scheduled`, `Computed`, `Validate`) and events (`BeforeInsert`, `AfterInsert`, `BeforeSave`, `AfterSave`, `BeforeDelete`, `AfterDelete`).

### Extension types

`ExtensionRecord` and `DeliveryRecord` structs for webhook extension management and delivery tracking.

## License

MIT
