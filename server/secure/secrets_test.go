package secure

import (
	"strings"
	"testing"
)

func TestEncryptRoundTrip(t *testing.T) {
	sealed, err := Encrypt("site-secret", "qiniu-access-key")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if !strings.HasPrefix(sealed, "enc:v1:") || strings.Contains(sealed, "qiniu-access-key") {
		t.Fatalf("密文格式不正确: %s", sealed)
	}
	plain, err := Decrypt("site-secret", sealed)
	if err != nil || plain != "qiniu-access-key" {
		t.Fatalf("解密失败: %q %v", plain, err)
	}
	if _, err := Decrypt("other-secret", sealed); err == nil {
		t.Fatal("使用不同站点密钥时不应能解密")
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	plain, err := Decrypt("site-secret", "legacy-key")
	if err != nil || plain != "legacy-key" {
		t.Fatalf("历史明文应兼容: %q %v", plain, err)
	}
}
