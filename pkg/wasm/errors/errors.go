package errors

import "errors"

const (
	ErrExit uint64 = iota + 1
	ErrInvalid
	ErrFile
	ErrMemoryAllocation
	ErrWrite
	ErrResourceNotFound
	ErrResourceNotAccessible
	ErrFileNotFound
	ErrDecodingJSON
	ErrDecodingYAML
	ErrEncodingJSON
	ErrEncodingYAML
	ErrConfig
)

var (
	ErrExitMessage                  = "error encountered"
	ErrInvalidMessage               = "invalid or missing arguments"
	ErrFileMessage                  = "could not read/write file"
	ErrWriteMessage                 = "could not write result"
	ErrMemoryAllocationMessage      = "could not allocate memory"
	ErrResourceNotFoundMessage      = "ocm resource not found"
	ErrResourceNotAccessibleMessage = "ocm resource not accessible"
	ErrFileNotFoundMessage          = "file not found"
	ErrDecodingJSONMessage          = "json is not valid"
	ErrDecodingYAMLMessage          = "yaml is not valid"
	ErrEncodingJSONMessage          = "json is not valid"
	ErrEncodingYAMLMessage          = "yaml is not valid"
	ErrConfigMessage                = "config is not valid"
)

// Check examines whether the exit code matches a defined error and if so
// returns an error type.
func Check(result []uint64) error {
	switch errNo := result[0]; errNo {
	case ErrExit:
		return errors.New(ErrExitMessage)
	case ErrInvalid:
		return errors.New(ErrInvalidMessage)
	case ErrFile:
		return errors.New(ErrFileMessage)
	case ErrMemoryAllocation:
		return errors.New(ErrMemoryAllocationMessage)
	case ErrWrite:
		return errors.New(ErrWriteMessage)
	case ErrResourceNotFound:
		return errors.New(ErrResourceNotFoundMessage)
	case ErrResourceNotAccessible:
		return errors.New(ErrResourceNotAccessibleMessage)
	case ErrFileNotFound:
		return errors.New(ErrFileNotFoundMessage)
	case ErrDecodingJSON:
		return errors.New(ErrDecodingJSONMessage)
	case ErrDecodingYAML:
		return errors.New(ErrDecodingYAMLMessage)
	case ErrEncodingJSON:
		return errors.New(ErrEncodingJSONMessage)
	case ErrEncodingYAML:
		return errors.New(ErrEncodingYAMLMessage)
	case ErrConfig:
		return errors.New(ErrConfigMessage)
	}

	return nil
}

// CheckCode examines whether the exit code matches an defined error and if so
// returns the WasmError.
func CheckCode(result []uint64) uint64 {
	switch err := result[0]; err {
	case ErrExit:
	case ErrInvalid:
	case ErrFile:
	case ErrMemoryAllocation:
	case ErrWrite:
	case ErrResourceNotFound:
	case ErrResourceNotAccessible:
	case ErrFileNotFound:
	case ErrDecodingJSON:
	case ErrDecodingYAML:
	case ErrEncodingJSON:
	case ErrEncodingYAML:
		return err
	}

	return 0
}

// GetMessage examines whether the exit code matches an defined error and if so
// returns the WasmError.
func GetMessage(err uint64) string {
	switch err {
	case ErrExit:
		return ErrExitMessage
	case ErrInvalid:
		return ErrInvalidMessage
	case ErrFile:
		return ErrFileMessage
	case ErrMemoryAllocation:
		return ErrMemoryAllocationMessage
	case ErrWrite:
		return ErrWriteMessage
	case ErrResourceNotFound:
		return ErrResourceNotFoundMessage
	case ErrResourceNotAccessible:
		return ErrResourceNotAccessibleMessage
	case ErrFileNotFound:
		return ErrFileNotFoundMessage
	case ErrDecodingJSON:
		return ErrDecodingJSONMessage
	case ErrDecodingYAML:
		return ErrDecodingYAMLMessage
	case ErrEncodingJSON:
		return ErrEncodingJSONMessage
	case ErrEncodingYAML:
		return ErrEncodingYAMLMessage
	case ErrConfig:
		return ErrConfigMessage
	}

	return "unknown error"
}
