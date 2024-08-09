package secrets

import (
	"encoding/base64"
	"strings"
)

func B64Encode(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func B64Decode(data string) string {
	decoded, _ := base64.StdEncoding.DecodeString(data)
	return string(decoded)
}

/*
ResolveVaultPrefix resolve vault prefix from vault ocid

	e.g. vault ocid: "ocid1.vault.oc1.ap-mumbai-1.ensluxzxaahi2.abrg6ljr4dfykdarhmr2urn3gopbrh53ahemqsa7wfmcmvgcrux3pwory6rq"
	     vault prefix: "ensluxzxaahi2"
*/
func ResolveVaultPrefix(vaultId string) string {
	if len(vaultId) <= 0 {
		return ""
	}
	vaultIdChunks := strings.Split(vaultId, ".")
	return vaultIdChunks[len(vaultIdChunks)-2]
}
