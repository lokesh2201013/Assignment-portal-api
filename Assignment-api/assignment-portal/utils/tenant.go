package utils

import "context"

type tenantContextKey struct{}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		tenantID = "default"
	}
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

func TenantIDFromContext(ctx context.Context) string {
	tenantID, ok := ctx.Value(tenantContextKey{}).(string)
	if !ok || tenantID == "" {
		return "default"
	}
	return tenantID
}
