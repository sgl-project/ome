package vibe

// Metadata is a json struct derived from locale.json
// Example data from a Vibe host that is in OC1 ca-toronto-1
//
//	 $ cat /opt/vibe/locale.json
//		{
//		 "ad_number_name": "ad1",
//		 "airport": "XXP",
//		 "availability_domain": "ad1",
//		 "bootstrapFootprint": "base.0-0",
//		 "id_code": "xp-1",
//		 "identity_realm": "rb1",
//		 "name": "sol-phoebe-1-ad-1",
//		 "realm": "rb1",
//		 "region": "sol-phoebe-1",
//		 "region_state": "Building",
//		 "service_enclave_dns_suffix": "svc.ad1.sol-phoebe-1"
//		}
type Metadata struct {
	Realm                   string `json:"realm"`                      // vibe target realm
	Region                  string `json:"region"`                     // vibe target region
	Airport                 string `json:"airport"`                    // vibe target region airport code
	ServiceEnclaveDNSSuffix string `json:"service_enclave_dns_suffix"` // vibe target service enclave dns suffix
	IdentityRealm           string `json:"identity_realm"`             // vibe identity realm
}
