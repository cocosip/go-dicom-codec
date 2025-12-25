# Obsolete Examples

The following example directories use deprecated APIs and need to be rewritten:

- `codec_usage.obsolete` - Uses old codec.Get, codec.List, codec.EncodeParams APIs
- `complete.obsolete` - Uses old codec.Get, baseline.Options, codec.BaseOptions APIs
- `jpegls_nearlossless.obsolete` - Uses old codec.Get, codec.EncodeParams, codec.BaseOptions APIs

These examples have been temporarily disabled (renamed with `.obsolete` suffix) until they can be updated to use the new codec API.

## New API Reference

- Use `codec.GetGlobalRegistry()` instead of `codec.Get()`
- Use `codec.Parameters` interface instead of `codec.EncodeParams`
- Use `imagetypes.PixelData` interface instead of `codec.PixelData` struct
- Codecs are accessed via registry: `registry.GetCodec(transferSyntax)`
