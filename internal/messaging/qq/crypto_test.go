package qq

import "testing"

func TestSignValidationPayloadOfficialSample(t *testing.T) {
	const (
		secret     = "DG5g3B4j9X2KOErG"
		eventTs    = "1725442341"
		plainToken = "Arq0D5A61EgUu4OxUvOp"
		expect     = "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d01bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706"
	)
	got := SignValidationPayload(secret, eventTs, plainToken)
	if got != expect {
		t.Fatalf("signature mismatch\n got: %s\nwant: %s", got, expect)
	}
}
