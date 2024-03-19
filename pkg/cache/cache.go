// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"io"
)

// Cache defines capabilities for a cache whatever the backing medium might be.
type Cache interface {
	IsCached(ctx context.Context, name, tag string) (bool, error)
	PushData(ctx context.Context, data io.ReadCloser, mediaType, name, tag string) (string, int64, error)
	FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, string, int64, error)
	FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error)
	DeleteData(ctx context.Context, name, tag string) error
}
