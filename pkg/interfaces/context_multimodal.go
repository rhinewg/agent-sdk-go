package interfaces

import "context"

// contextKey is a private type to avoid key collisions across packages.
type contextKey string

const multimodalContentPartsKey contextKey = "interfaces.multimodal.content_parts"

// WithContextContentParts attaches multimodal content parts to the context.
// This enables passing multimodal input through existing APIs without
// changing method signatures (backward compatible).
func WithContextContentParts(ctx context.Context, parts ...ContentPart) context.Context {
	if len(parts) == 0 {
		return ctx
	}
	return context.WithValue(ctx, multimodalContentPartsKey, parts)
}

// GetContextContentParts retrieves multimodal content parts from the context.
func GetContextContentParts(ctx context.Context) ([]ContentPart, bool) {
	parts, ok := ctx.Value(multimodalContentPartsKey).([]ContentPart)
	return parts, ok
}

// ClearContextContentParts explicitly overrides multimodal content parts in the context
// with an empty slice.
//
// This is useful when you want to run follow-up reasoning/tool-calling on the text
// only, while still allowing an earlier step (e.g. a vision extractor) to consume
// the images from the original context.
func ClearContextContentParts(ctx context.Context) context.Context {
	return context.WithValue(ctx, multimodalContentPartsKey, []ContentPart{})
}

