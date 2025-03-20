package env

func DefaultCanonicalRegionNames() map[string]string {
	return map[string]string{
		// region1
		// r1 is an exception where "r1" is the official name and "sea" is occasionally used
		"sea":          "r1",
		"us-seattle-1": "r1",

		// oc0
		"rnt": "us-renton-1",
		"scf": "us-scottsdale-1",

		// oc1
		"r2":  "us-phoenix-1",
		"phx": "us-phoenix-1",
		"iad": "us-ashburn-1",
		"fra": "eu-frankfurt-1",
		"lhr": "uk-london-1",
		"yyz": "ca-toronto-1",
		"nrt": "ap-tokyo-1",
		"icn": "ap-seoul-1",
		"bom": "ap-mumbai-1",
		"kix": "ap-osaka-1",
		"zrh": "eu-zurich-1",
		"syd": "ap-sydney-1",
		"gru": "sa-saopaulo-1",
		"ams": "eu-amsterdam-1",
		"jed": "me-jeddah-1",
		"mel": "ap-melbourne-1",
		"yul": "ca-montreal-1",
		"hyd": "ap-hyderabad-1",
		"yny": "ap-chuncheon-1",
		"sjc": "us-sanjose-1",
		"dxb": "me-dubai-1",
		"cwl": "uk-cardiff-1",
		"sin": "ap-singapore-1",
		"auh": "me-abudhabi-1",
		"scl": "sa-santiago-1",
		"vcp": "sa-vinhedo-1",
		"lin": "eu-milan-1",
		"arn": "eu-stockholm-1",
		"mrs": "eu-marseille-1",
		"jnb": "af-johannesburg-1",
		"mtz": "il-jerusalem-1",
		"cdg": "eu-paris-1",
		"qro": "mx-queretaro-1",
		"lke": "us-lke-1",
		"ord": "us-chicago-1",

		// oc2
		"lfi": "us-langley-1",
		"luf": "us-luke-1",

		// oc3
		"ric": "us-gov-ashburn-1",
		"tus": "us-gov-phoenix-1",
		"pia": "us-gov-chicago-1",

		// oc4
		"ltn": "uk-gov-london-1",
		"brs": "uk-gov-cardiff-1",

		// oc5
		"tiw": "us-tacoma-1",

		// oc6
		"ftw": "us-gov-fortworth-1",
		"dca": "us-gov-sterling-2",

		// oc7
		"bwi": "us-gov-sterling-1",

		// oc8
		"nja": "ap-chiyoda-1",
		"ukb": "ap-ibaraki-1",

		// oc9
		"mct": "me-dcc-muscat-1",

		// oc10
		"wga": "ap-dcc-canberra-1",

		// oc11
		"pit": "us-gov-boyers-1",
		"dal": "us-gov-fortworth-3",
		"gyr": "us-gov-phoenix-3",
		"hef": "us-gov-sterling-3",

		// oc12
		"geu": "us-gov-phoenix-2",
		"okv": "us-gov-ashburn-2",
		"slc": "us-gov-saltlakecity-1", // deprecated
		"vel": "us-gov-saltlake-1",
		"hwy": "us-gov-manassas-1",

		// oc14
		"bgy": "eu-dcc-milan-1",
	}
}

func DefaultRealmConfigs() map[string]*RealmConfig {
	return map[string]*RealmConfig{
		"test": {
			Regions: []string{"test"},
		},
		"region1": {
			Regions: []string{"r1"},
		},

		"oc0": {
			Regions: []string{
				"us-renton-1",
				"us-scottsdale-1",
			},
		},

		"oc1": {
			Regions: []string{
				"us-phoenix-1",
				"us-ashburn-1",
				"eu-frankfurt-1",
				"uk-london-1",
				"ca-toronto-1",
				"ap-tokyo-1",
				"ap-seoul-1",
				"ap-mumbai-1",
				"ap-osaka-1",
				"eu-zurich-1",
				"ap-sydney-1",
				"sa-saopaulo-1",
				"eu-amsterdam-1",
				"me-jeddah-1",
				"ap-melbourne-1",
				"ca-montreal-1",
				"ap-hyderabad-1",
				"ap-chuncheon-1",
				"us-sanjose-1",
				"me-dubai-1",
				"uk-cardiff-1",
				"ap-singapore-1",
				"me-abudhabi-1",
				"sa-santiago-1",
				"sa-vinhedo-1",
				"eu-milan-1",
				"eu-stockholm-1",
				"eu-marseille-1",
				"af-johannesburg-1",
				"il-jerusalem-1",
				"eu-paris-1",
				"mx-queretaro-1",
				"us-lke-1",
				"us-chicago-1",
			},
		},

		// GOV
		"oc2": {
			IsGov: true,
			Regions: []string{
				"us-langley-1",
				"us-luke-1",
			},
		},
		"oc3": {
			IsGov: true,
			Regions: []string{
				"us-gov-ashburn-1",
				"us-gov-phoenix-1",
				"us-gov-chicago-1",
			},
		},
		"oc4": {
			IsGov: true,
			Regions: []string{
				"uk-gov-london-1",
				"uk-gov-cardiff-1",
			},
		},

		// ONSR
		"oc5": {
			IsONSR: true,
			Regions: []string{
				"us-tacoma-1",
			},
		},
		"oc6": {
			IsONSR: true,
			Regions: []string{
				"us-gov-fortworth-1",
				"us-gov-sterling-2",
			},
		},
		"oc7": {
			IsONSR: true,
			Regions: []string{
				"us-gov-sterling-1",
			},
		},

		"oc8": {
			Regions: []string{
				"ap-chiyoda-1",
				"ap-ibaraki-1",
			},
		},
		"oc9": {
			Regions: []string{
				"me-dcc-muscat-1",
			},
		},
		"oc10": {
			Regions: []string{
				"ap-dcc-canberra-1",
			},
		},
		"oc11": {
			IsONSR: true,
			Regions: []string{
				"us-gov-fortworth-3",
				"us-gov-boyers-1",
				"us-gov-phoenix-3",
				"us-gov-sterling-3",
			},
		},
		"oc12": {
			IsONSR: true,
			Regions: []string{
				"us-gov-phoenix-2",
				"us-gov-ashburn-2",
				"us-gov-manassas-1",

				// us-gov-saltlakecity-1 is being deprecated in favor of us-gov-saltlake-1
				// (due to region-length issues):
				// https://dyn.slack.com/archives/CA11X4SE8/p1638404855496700?thread_ts=1637788311.110000&cid=CA11X4SE8
				"us-gov-saltlakecity-1",
				"us-gov-saltlake-1",
			},
		},
		"oc14": {
			Regions: []string{
				"eu-dcc-milan-1",
			},
		},
	}
}
