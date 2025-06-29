package electricitymaps_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/michaelpeterswa/lfpweather-api/pkg/electricitymaps"
)

func TestGetZones(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     *string
		mockServer func() *httptest.Server
		expectErr  bool
	}{
		{
			name:   "Valid response",
			apiKey: nil,
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"zone1": {"countryName": "Country1", "zoneName": "Zone1", "displayName": "Display1", "access": "Access1"}}`))
				}))
			},
			expectErr: false,
		},
		{
			name:   "Error response",
			apiKey: nil,
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectErr: true,
		},
		{
			name:   "Full Response June 1 2025",
			apiKey: nil,
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{
	"AD": {
		"zoneName": "Andorra"
	},
	"AE": {
		"zoneName": "United Arab Emirates"
	},
	"AF": {
		"zoneName": "Afghanistan"
	},
	"AG": {
		"zoneName": "Antigua and Barbuda"
	},
	"AL": {
		"zoneName": "Albania"
	},
	"AM": {
		"zoneName": "Armenia"
	},
	"AO": {
		"zoneName": "Angola"
	},
	"AR": {
		"zoneName": "Argentina"
	},
	"AT": {
		"zoneName": "Austria"
	},
	"AU": {
		"zoneName": "Australia"
	},
	"AU-LH": {
		"countryName": "Australia",
		"zoneName": "Lord Howe Island"
	},
	"AU-NSW": {
		"countryName": "Australia",
		"zoneName": "New South Wales"
	},
	"AU-NT": {
		"countryName": "Australia",
		"zoneName": "Northern Territory"
	},
	"AU-QLD": {
		"countryName": "Australia",
		"zoneName": "Queensland"
	},
	"AU-SA": {
		"countryName": "Australia",
		"zoneName": "South Australia"
	},
	"AU-TAS": {
		"countryName": "Australia",
		"zoneName": "Tasmania"
	},
	"AU-TAS-CBI": {
		"countryName": "Australia",
		"zoneName": "Cape Barren Island"
	},
	"AU-TAS-FI": {
		"countryName": "Australia",
		"zoneName": "Flinders Island"
	},
	"AU-TAS-KI": {
		"countryName": "Australia",
		"zoneName": "King Island"
	},
	"AU-VIC": {
		"countryName": "Australia",
		"zoneName": "Victoria"
	},
	"AU-WA": {
		"countryName": "Australia",
		"zoneName": "Western Australia"
	},
	"AU-WA-RI": {
		"countryName": "Australia",
		"zoneName": "Rottnest Island"
	},
	"AW": {
		"zoneName": "Aruba"
	},
	"AX": {
		"zoneName": "Åland Islands"
	},
	"AZ": {
		"zoneName": "Azerbaijan"
	},
	"BA": {
		"zoneName": "Bosnia and Herzegovina"
	},
	"BB": {
		"zoneName": "Barbados"
	},
	"BD": {
		"zoneName": "Bangladesh"
	},
	"BE": {
		"zoneName": "Belgium"
	},
	"BF": {
		"zoneName": "Burkina Faso"
	},
	"BG": {
		"zoneName": "Bulgaria"
	},
	"BH": {
		"zoneName": "Bahrain"
	},
	"BI": {
		"zoneName": "Burundi"
	},
	"BJ": {
		"zoneName": "Benin"
	},
	"BM": {
		"zoneName": "Bermuda"
	},
	"BN": {
		"zoneName": "Brunei"
	},
	"BO": {
		"zoneName": "Bolivia"
	},
	"BR": {
		"zoneName": "Brazil"
	},
	"BR-CS": {
		"countryName": "Brazil",
		"zoneName": "Central Brazil"
	},
	"BR-N": {
		"countryName": "Brazil",
		"zoneName": "North Brazil"
	},
	"BR-NE": {
		"countryName": "Brazil",
		"zoneName": "North-East Brazil"
	},
	"BR-S": {
		"countryName": "Brazil",
		"zoneName": "South Brazil"
	},
	"BS": {
		"zoneName": "Bahamas"
	},
	"BT": {
		"zoneName": "Bhutan"
	},
	"BW": {
		"zoneName": "Botswana"
	},
	"BY": {
		"zoneName": "Belarus"
	},
	"BZ": {
		"zoneName": "Belize"
	},
	"CA": {
		"zoneName": "Canada"
	},
	"CA-AB": {
		"countryName": "Canada",
		"zoneName": "Alberta"
	},
	"CA-BC": {
		"countryName": "Canada",
		"zoneName": "British Columbia"
	},
	"CA-MB": {
		"countryName": "Canada",
		"zoneName": "Manitoba"
	},
	"CA-NB": {
		"countryName": "Canada",
		"zoneName": "New Brunswick"
	},
	"CA-NL": {
		"countryName": "Canada",
		"zoneName": "Newfoundland and Labrador"
	},
	"CA-NS": {
		"countryName": "Canada",
		"zoneName": "Nova Scotia"
	},
	"CA-NT": {
		"countryName": "Canada",
		"zoneName": "Northwest Territories"
	},
	"CA-NU": {
		"countryName": "Canada",
		"zoneName": "Nunavut"
	},
	"CA-ON": {
		"countryName": "Canada",
		"zoneName": "Ontario"
	},
	"CA-PE": {
		"countryName": "Canada",
		"zoneName": "Prince Edward Island"
	},
	"CA-QC": {
		"countryName": "Canada",
		"zoneName": "Québec"
	},
	"CA-SK": {
		"countryName": "Canada",
		"zoneName": "Saskatchewan"
	},
	"CA-YT": {
		"countryName": "Canada",
		"zoneName": "Yukon"
	},
	"CD": {
		"zoneName": "Democratic Republic of the Congo"
	},
	"CF": {
		"zoneName": "Central African Republic"
	},
	"CG": {
		"zoneName": "Congo"
	},
	"CH": {
		"zoneName": "Switzerland"
	},
	"CI": {
		"zoneName": "Ivory Coast"
	},
	"CL-CHP": {
		"countryName": "Chile",
		"zoneName": "Easter Island"
	},
	"CL-SEA": {
		"countryName": "Chile",
		"zoneName": "Sistema Eléctrico de Aysén"
	},
	"CL-SEM": {
		"countryName": "Chile",
		"zoneName": "Sistema Eléctrico de Magallanes"
	},
	"CL-SEN": {
		"countryName": "Chile",
		"zoneName": "Sistema Eléctrico Nacional"
	},
	"CM": {
		"zoneName": "Cameroon"
	},
	"CN": {
		"zoneName": "China"
	},
	"CO": {
		"zoneName": "Colombia"
	},
	"CR": {
		"zoneName": "Costa Rica"
	},
	"CU": {
		"zoneName": "Cuba"
	},
	"CV": {
		"zoneName": "Cabo Verde"
	},
	"CW": {
		"zoneName": "Curaçao"
	},
	"CY": {
		"zoneName": "Cyprus"
	},
	"CZ": {
		"zoneName": "Czechia"
	},
	"DE": {
		"zoneName": "Germany"
	},
	"DJ": {
		"zoneName": "Djibouti"
	},
	"DK": {
		"zoneName": "Denmark"
	},
	"DK-BHM": {
		"countryName": "Denmark",
		"zoneName": "Bornholm"
	},
	"DK-DK1": {
		"countryName": "Denmark",
		"zoneName": "West Denmark"
	},
	"DK-DK2": {
		"countryName": "Denmark",
		"zoneName": "East Denmark"
	},
	"DM": {
		"zoneName": "Dominica"
	},
	"DO": {
		"zoneName": "Dominican Republic"
	},
	"DZ": {
		"zoneName": "Algeria"
	},
	"EC": {
		"zoneName": "Ecuador"
	},
	"EE": {
		"zoneName": "Estonia"
	},
	"EG": {
		"zoneName": "Egypt"
	},
	"EH": {
		"zoneName": "Western Sahara"
	},
	"ER": {
		"zoneName": "Eritrea"
	},
	"ES": {
		"zoneName": "Spain"
	},
	"ES-CE": {
		"countryName": "Spain",
		"zoneName": "Ceuta"
	},
	"ES-CN-FV": {
		"countryName": "Spain",
		"zoneName": "Fuerteventura"
	},
	"ES-CN-GC": {
		"countryName": "Spain",
		"zoneName": "Gran Canaria"
	},
	"ES-CN-HI": {
		"countryName": "Spain",
		"zoneName": "El Hierro"
	},
	"ES-CN-IG": {
		"countryName": "Spain",
		"zoneName": "Isla de la Gomera"
	},
	"ES-CN-LP": {
		"countryName": "Spain",
		"zoneName": "La Palma"
	},
	"ES-CN-LZ": {
		"countryName": "Spain",
		"zoneName": "Lanzarote"
	},
	"ES-CN-TE": {
		"countryName": "Spain",
		"zoneName": "Tenerife"
	},
	"ES-IB-FO": {
		"countryName": "Spain",
		"zoneName": "Formentera"
	},
	"ES-IB-IZ": {
		"countryName": "Spain",
		"zoneName": "Ibiza"
	},
	"ES-IB-MA": {
		"countryName": "Spain",
		"zoneName": "Mallorca"
	},
	"ES-IB-ME": {
		"countryName": "Spain",
		"zoneName": "Menorca"
	},
	"ES-ML": {
		"countryName": "Spain",
		"zoneName": "Melilla"
	},
	"ET": {
		"zoneName": "Ethiopia"
	},
	"FI": {
		"zoneName": "Finland"
	},
	"FJ": {
		"zoneName": "Fiji"
	},
	"FK": {
		"zoneName": "Falkland Islands"
	},
	"FM": {
		"zoneName": "Micronesia"
	},
	"FO": {
		"zoneName": "Faroe Islands"
	},
	"FO-MI": {
		"countryName": "Faroe Islands",
		"zoneName": "Main Islands"
	},
	"FO-SI": {
		"countryName": "Faroe Islands",
		"zoneName": "South Island"
	},
	"FR": {
		"zoneName": "France"
	},
	"FR-COR": {
		"countryName": "France",
		"zoneName": "Corsica"
	},
	"GA": {
		"zoneName": "Gabon"
	},
	"GB": {
		"zoneName": "Great Britain"
	},
	"GB-NIR": {
		"zoneName": "Northern Ireland"
	},
	"GB-ORK": {
		"countryName": "Great Britain",
		"zoneName": "Orkney Islands"
	},
	"GB-ZET": {
		"countryName": "Great Britain",
		"zoneName": "Shetland Islands"
	},
	"GE": {
		"zoneName": "Georgia"
	},
	"GF": {
		"zoneName": "French Guiana"
	},
	"GG": {
		"zoneName": "Guernsey"
	},
	"GH": {
		"zoneName": "Ghana"
	},
	"GI": {
		"zoneName": "Gibraltar"
	},
	"GL": {
		"zoneName": "Greenland"
	},
	"GM": {
		"zoneName": "Gambia"
	},
	"GN": {
		"zoneName": "Guinea"
	},
	"GP": {
		"zoneName": "Guadeloupe"
	},
	"GQ": {
		"zoneName": "Equatorial Guinea"
	},
	"GR": {
		"zoneName": "Greece"
	},
	"GS": {
		"zoneName": "South Georgia and the South Sandwich Islands"
	},
	"GT": {
		"zoneName": "Guatemala"
	},
	"GU": {
		"zoneName": "Guam"
	},
	"GW": {
		"zoneName": "Guinea-Bissau"
	},
	"GY": {
		"zoneName": "Guyana"
	},
	"HK": {
		"zoneName": "Hong Kong"
	},
	"HM": {
		"zoneName": "Heard Island and McDonald Islands"
	},
	"HN": {
		"zoneName": "Honduras"
	},
	"HR": {
		"zoneName": "Croatia"
	},
	"HT": {
		"zoneName": "Haiti"
	},
	"HU": {
		"zoneName": "Hungary"
	},
	"ID": {
		"zoneName": "Indonesia"
	},
	"IE": {
		"zoneName": "Ireland"
	},
	"IL": {
		"zoneName": "Israel"
	},
	"IM": {
		"zoneName": "Isle of Man"
	},
	"IN": {
		"zoneName": "Mainland India"
	},
	"IN-AN": {
		"countryName": "India",
		"zoneName": "Andaman and Nicobar Islands"
	},
	"IN-DL": {
		"zoneName": "Unknown"
	},
	"IN-EA": {
		"countryName": "India",
		"zoneName": "Eastern India"
	},
	"IN-HP": {
		"countryName": "India",
		"zoneName": "Himachal Pradesh"
	},
	"IN-KA": {
		"zoneName": "Unknown"
	},
	"IN-MH": {
		"zoneName": "Unknown"
	},
	"IN-NE": {
		"countryName": "India",
		"zoneName": "North Eastern India"
	},
	"IN-NO": {
		"countryName": "India",
		"zoneName": "Northern India"
	},
	"IN-PB": {
		"zoneName": "Unknown"
	},
	"IN-SO": {
		"countryName": "India",
		"zoneName": "Southern India"
	},
	"IN-UP": {
		"countryName": "India",
		"zoneName": "Uttar Pradesh"
	},
	"IN-UT": {
		"countryName": "India",
		"zoneName": "Uttarakhand"
	},
	"IN-WE": {
		"countryName": "India",
		"zoneName": "Western India"
	},
	"IQ": {
		"zoneName": "Iraq"
	},
	"IR": {
		"zoneName": "Iran"
	},
	"IS": {
		"zoneName": "Iceland"
	},
	"IT": {
		"zoneName": "Italy"
	},
	"IT-CNO": {
		"countryName": "Italy",
		"zoneName": "Central North Italy"
	},
	"IT-CSO": {
		"countryName": "Italy",
		"zoneName": "Central South Italy"
	},
	"IT-NO": {
		"countryName": "Italy",
		"zoneName": "North Italy"
	},
	"IT-SAR": {
		"countryName": "Italy",
		"zoneName": "Sardinia"
	},
	"IT-SIC": {
		"countryName": "Italy",
		"zoneName": "Sicily"
	},
	"IT-SO": {
		"countryName": "Italy",
		"zoneName": "South Italy"
	},
	"JE": {
		"zoneName": "Jersey"
	},
	"JM": {
		"zoneName": "Jamaica"
	},
	"JO": {
		"zoneName": "Jordan"
	},
	"JP": {
		"zoneName": "Japan"
	},
	"JP-CB": {
		"countryName": "Japan",
		"zoneName": "Chūbu"
	},
	"JP-CG": {
		"countryName": "Japan",
		"zoneName": "Chūgoku"
	},
	"JP-HKD": {
		"countryName": "Japan",
		"zoneName": "Hokkaidō"
	},
	"JP-HR": {
		"countryName": "Japan",
		"zoneName": "Hokuriku"
	},
	"JP-KN": {
		"countryName": "Japan",
		"zoneName": "Kansai"
	},
	"JP-KY": {
		"countryName": "Japan",
		"zoneName": "Kyūshū"
	},
	"JP-ON": {
		"countryName": "Japan",
		"zoneName": "Okinawa"
	},
	"JP-SK": {
		"countryName": "Japan",
		"zoneName": "Shikoku"
	},
	"JP-TH": {
		"countryName": "Japan",
		"zoneName": "Tōhoku"
	},
	"JP-TK": {
		"countryName": "Japan",
		"zoneName": "Tōkyō"
	},
	"KE": {
		"zoneName": "Kenya"
	},
	"KG": {
		"zoneName": "Kyrgyzstan"
	},
	"KH": {
		"zoneName": "Cambodia"
	},
	"KM": {
		"zoneName": "Comoros"
	},
	"KP": {
		"zoneName": "North Korea"
	},
	"KR": {
		"zoneName": "South Korea"
	},
	"KW": {
		"zoneName": "Kuwait"
	},
	"KY": {
		"zoneName": "Cayman Islands"
	},
	"KZ": {
		"zoneName": "Kazakhstan"
	},
	"LA": {
		"zoneName": "Laos"
	},
	"LB": {
		"zoneName": "Lebanon"
	},
	"LC": {
		"zoneName": "Saint Lucia"
	},
	"LI": {
		"zoneName": "Liechtenstein"
	},
	"LK": {
		"zoneName": "Sri Lanka"
	},
	"LR": {
		"zoneName": "Liberia"
	},
	"LS": {
		"zoneName": "Lesotho"
	},
	"LT": {
		"zoneName": "Lithuania"
	},
	"LU": {
		"zoneName": "Luxembourg"
	},
	"LV": {
		"zoneName": "Latvia"
	},
	"LY": {
		"zoneName": "Libya"
	},
	"MA": {
		"zoneName": "Morocco"
	},
	"MC": {
		"zoneName": "Monaco"
	},
	"MD": {
		"zoneName": "Moldova"
	},
	"ME": {
		"zoneName": "Montenegro"
	},
	"MG": {
		"zoneName": "Madagascar"
	},
	"MK": {
		"zoneName": "North Macedonia"
	},
	"ML": {
		"zoneName": "Mali"
	},
	"MM": {
		"zoneName": "Myanmar"
	},
	"MN": {
		"zoneName": "Mongolia"
	},
	"MO": {
		"zoneName": "Macao"
	},
	"MQ": {
		"zoneName": "Martinique"
	},
	"MR": {
		"zoneName": "Mauritania"
	},
	"MT": {
		"zoneName": "Malta"
	},
	"MU": {
		"zoneName": "Mauritius"
	},
	"MV": {
		"zoneName": "Maldives"
	},
	"MW": {
		"zoneName": "Malawi"
	},
	"MX": {
		"zoneName": "Mexico"
	},
	"MY": {
		"zoneName": "Malaysia"
	},
	"MY-EM": {
		"countryName": "Malaysia",
		"zoneName": "Borneo"
	},
	"MY-WM": {
		"countryName": "Malaysia",
		"zoneName": "Peninsula"
	},
	"MZ": {
		"zoneName": "Mozambique"
	},
	"NA": {
		"zoneName": "Namibia"
	},
	"NC": {
		"zoneName": "New Caledonia"
	},
	"NE": {
		"zoneName": "Niger"
	},
	"NG": {
		"zoneName": "Nigeria"
	},
	"NI": {
		"zoneName": "Nicaragua"
	},
	"NL": {
		"zoneName": "Netherlands"
	},
	"NO": {
		"zoneName": "Norway"
	},
	"NO-NO1": {
		"countryName": "Norway",
		"zoneName": "Southeast Norway"
	},
	"NO-NO2": {
		"countryName": "Norway",
		"zoneName": "Southwest Norway"
	},
	"NO-NO3": {
		"countryName": "Norway",
		"zoneName": "Middle Norway"
	},
	"NO-NO4": {
		"countryName": "Norway",
		"zoneName": "North Norway"
	},
	"NO-NO5": {
		"countryName": "Norway",
		"zoneName": "West Norway"
	},
	"NP": {
		"zoneName": "Nepal"
	},
	"NZ": {
		"zoneName": "New Zealand"
	},
	"NZ-NZA": {
		"countryName": "New Zealand",
		"zoneName": "Auckland Islands"
	},
	"NZ-NZC": {
		"countryName": "New Zealand",
		"zoneName": "Chatham Islands"
	},
	"NZ-NZST": {
		"countryName": "New Zealand",
		"zoneName": "Stewart Island"
	},
	"OM": {
		"zoneName": "Oman"
	},
	"PA": {
		"zoneName": "Panama"
	},
	"PE": {
		"zoneName": "Peru"
	},
	"PF": {
		"zoneName": "French Polynesia"
	},
	"PG": {
		"zoneName": "Papua New Guinea"
	},
	"PH": {
		"zoneName": "Philippines"
	},
	"PH-LU": {
		"countryName": "Philippines",
		"zoneName": "Luzon"
	},
	"PH-MI": {
		"countryName": "Philippines",
		"zoneName": "Mindanao"
	},
	"PH-VI": {
		"countryName": "Philippines",
		"zoneName": "Visayas"
	},
	"PK": {
		"zoneName": "Pakistan"
	},
	"PL": {
		"zoneName": "Poland"
	},
	"PM": {
		"zoneName": "Saint Pierre and Miquelon"
	},
	"PR": {
		"zoneName": "Puerto Rico"
	},
	"PS": {
		"zoneName": "State of Palestine"
	},
	"PT": {
		"zoneName": "Portugal"
	},
	"PT-AC": {
		"countryName": "Portugal",
		"zoneName": "Azores"
	},
	"PT-MA": {
		"countryName": "Portugal",
		"zoneName": "Madeira"
	},
	"PW": {
		"zoneName": "Palau"
	},
	"PY": {
		"zoneName": "Paraguay"
	},
	"QA": {
		"zoneName": "Qatar"
	},
	"RE": {
		"zoneName": "Réunion"
	},
	"RO": {
		"zoneName": "Romania"
	},
	"RS": {
		"zoneName": "Serbia"
	},
	"RU": {
		"zoneName": "Russia"
	},
	"RU-1": {
		"countryName": "Russia",
		"zoneName": "Europe-Ural"
	},
	"RU-2": {
		"countryName": "Russia",
		"zoneName": "Siberia"
	},
	"RU-AS": {
		"countryName": "Russia",
		"zoneName": "East"
	},
	"RU-EU": {
		"countryName": "Russia",
		"zoneName": "Arctic"
	},
	"RU-FE": {
		"countryName": "Russia",
		"zoneName": "Far East"
	},
	"RU-KGD": {
		"countryName": "Russia",
		"zoneName": "Kaliningrad"
	},
	"RW": {
		"zoneName": "Rwanda"
	},
	"SA": {
		"zoneName": "Saudi Arabia"
	},
	"SB": {
		"zoneName": "Solomon Islands"
	},
	"SC": {
		"zoneName": "Seychelles"
	},
	"SD": {
		"zoneName": "Sudan"
	},
	"SE": {
		"zoneName": "Sweden"
	},
	"SE-SE1": {
		"countryName": "Sweden",
		"zoneName": "North Sweden"
	},
	"SE-SE2": {
		"countryName": "Sweden",
		"zoneName": "North Central Sweden"
	},
	"SE-SE3": {
		"countryName": "Sweden",
		"zoneName": "South Central Sweden"
	},
	"SE-SE4": {
		"countryName": "Sweden",
		"zoneName": "South Sweden"
	},
	"SG": {
		"zoneName": "Singapore"
	},
	"SI": {
		"zoneName": "Slovenia"
	},
	"SJ": {
		"zoneName": "Svalbard and Jan Mayen"
	},
	"SK": {
		"zoneName": "Slovakia"
	},
	"SL": {
		"zoneName": "Sierra Leone"
	},
	"SN": {
		"zoneName": "Senegal"
	},
	"SO": {
		"zoneName": "Somalia"
	},
	"SR": {
		"zoneName": "Suriname"
	},
	"SS": {
		"zoneName": "South Sudan"
	},
	"ST": {
		"zoneName": "São Tomé and Príncipe"
	},
	"SV": {
		"zoneName": "El Salvador"
	},
	"SY": {
		"zoneName": "Syria"
	},
	"SZ": {
		"zoneName": "Eswatini"
	},
	"TD": {
		"zoneName": "Chad"
	},
	"TF": {
		"zoneName": "French Southern Territories"
	},
	"TG": {
		"zoneName": "Togo"
	},
	"TH": {
		"zoneName": "Thailand"
	},
	"TJ": {
		"zoneName": "Tajikistan"
	},
	"TL": {
		"zoneName": "Timor-Leste"
	},
	"TM": {
		"zoneName": "Turkmenistan"
	},
	"TN": {
		"zoneName": "Tunisia"
	},
	"TO": {
		"zoneName": "Tonga"
	},
	"TR": {
		"zoneName": "Turkey"
	},
	"TT": {
		"zoneName": "Trinidad and Tobago"
	},
	"TW": {
		"zoneName": "Taiwan"
	},
	"TZ": {
		"zoneName": "Tanzania"
	},
	"UA": {
		"zoneName": "Ukraine"
	},
	"UA-CR": {
		"countryName": "Ukraine",
		"zoneName": "Crimea"
	},
	"UG": {
		"zoneName": "Uganda"
	},
	"US": {
		"zoneName": "United States"
	},
	"US-AK": {
		"countryName": "USA",
		"zoneName": "Alaska"
	},
	"US-AK-SEAPA": {
		"countryName": "USA",
		"zoneName": "Southeast Alaska Power Agency"
	},
	"US-CAL-BANC": {
		"countryName": "USA",
		"displayName": "BANC",
		"zoneName": "Balancing Authority of Northern California"
	},
	"US-CAL-CISO": {
		"countryName": "USA",
		"displayName": "California ISO",
		"zoneName": "CAISO"
	},
	"US-CAL-IID": {
		"countryName": "USA",
		"zoneName": "Imperial Irrigation District"
	},
	"US-CAL-LDWP": {
		"countryName": "USA",
		"zoneName": "Los Angeles Department of Water and Power"
	},
	"US-CAL-TIDC": {
		"countryName": "USA",
		"zoneName": "Turlock Irrigation District"
	},
	"US-CAR-CPLE": {
		"countryName": "USA",
		"zoneName": "Duke Energy Progress East"
	},
	"US-CAR-CPLW": {
		"countryName": "USA",
		"zoneName": "Duke Energy Progress West"
	},
	"US-CAR-DUK": {
		"countryName": "USA",
		"zoneName": "Duke Energy Carolinas"
	},
	"US-CAR-SC": {
		"countryName": "USA",
		"zoneName": "South Carolina Public Service Authority"
	},
	"US-CAR-SCEG": {
		"countryName": "USA",
		"zoneName": "South Carolina Electric & Gas Company"
	},
	"US-CAR-YAD": {
		"countryName": "USA",
		"displayName": "Alcoa Power Generating",
		"zoneName": "Alcoa Power Generating, Inc. Yadkin Division"
	},
	"US-CENT-SPA": {
		"countryName": "USA",
		"zoneName": "Southwestern Power Administration"
	},
	"US-CENT-SWPP": {
		"countryName": "USA",
		"displayName": "SPP",
		"zoneName": "Southwest Power Pool"
	},
	"US-FLA-FMPP": {
		"countryName": "USA",
		"zoneName": "Florida Municipal Power Pool"
	},
	"US-FLA-FPC": {
		"countryName": "USA",
		"zoneName": "Duke Energy Florida"
	},
	"US-FLA-FPL": {
		"countryName": "USA",
		"zoneName": "Florida Power and Light Company"
	},
	"US-FLA-GVL": {
		"countryName": "USA",
		"zoneName": "Gainesville Regional Utilities"
	},
	"US-FLA-HST": {
		"countryName": "USA",
		"zoneName": "City of Homestead"
	},
	"US-FLA-JEA": {
		"countryName": "USA",
		"zoneName": "Jacksonville Electric Authority"
	},
	"US-FLA-SEC": {
		"countryName": "USA",
		"zoneName": "Seminole Electric Cooperative"
	},
	"US-FLA-TAL": {
		"countryName": "USA",
		"zoneName": "City of Tallahassee"
	},
	"US-FLA-TEC": {
		"countryName": "USA",
		"zoneName": "Tampa Electric Company"
	},
	"US-HI": {
		"countryName": "USA",
		"zoneName": "Hawaii"
	},
	"US-MIDA-PJM": {
		"countryName": "USA",
		"displayName": "PJM",
		"zoneName": "PJM Interconnection"
	},
	"US-MIDW-AECI": {
		"countryName": "USA",
		"zoneName": "Associated Electric Cooperative"
	},
	"US-MIDW-LGEE": {
		"countryName": "USA",
		"displayName": "Louisville Gas And Electric Company",
		"zoneName": "Louisville Gas and Electric Company and Kentucky Utilities"
	},
	"US-MIDW-MISO": {
		"countryName": "USA",
		"displayName": "MISO",
		"zoneName": "Midcontinent ISO"
	},
	"US-NE-ISNE": {
		"countryName": "USA",
		"zoneName": "ISO New England"
	},
	"US-NW-AVA": {
		"countryName": "USA",
		"zoneName": "Avista Corporation"
	},
	"US-NW-BPAT": {
		"countryName": "USA",
		"zoneName": "Bonneville Power Administration"
	},
	"US-NW-CHPD": {
		"countryName": "USA",
		"zoneName": "Chelan County"
	},
	"US-NW-DOPD": {
		"countryName": "USA",
		"zoneName": "Douglas County"
	},
	"US-NW-GCPD": {
		"countryName": "USA",
		"zoneName": "Grant County"
	},
	"US-NW-GRID": {
		"countryName": "USA",
		"zoneName": "Gridforce Energy Management"
	},
	"US-NW-IPCO": {
		"countryName": "USA",
		"zoneName": "Idaho Power Company"
	},
	"US-NW-NEVP": {
		"countryName": "USA",
		"zoneName": "Nevada Power Company"
	},
	"US-NW-NWMT": {
		"countryName": "USA",
		"zoneName": "Northwestern Energy"
	},
	"US-NW-PACE": {
		"countryName": "USA",
		"zoneName": "Pacificorp East"
	},
	"US-NW-PACW": {
		"countryName": "USA",
		"zoneName": "Pacificorp West"
	},
	"US-NW-PGE": {
		"countryName": "USA",
		"zoneName": "Portland General Electric Company"
	},
	"US-NW-PSCO": {
		"countryName": "USA",
		"displayName": "PSCo",
		"zoneName": "Public Service Company of Colorado"
	},
	"US-NW-PSEI": {
		"countryName": "USA",
		"zoneName": "Puget Sound Energy"
	},
	"US-NW-SCL": {
		"countryName": "USA",
		"zoneName": "Seattle City Light"
	},
	"US-NW-TPWR": {
		"countryName": "USA",
		"zoneName": "City of Tacoma"
	},
	"US-NW-WACM": {
		"countryName": "USA",
		"displayName": "WAPA Rocky Mountains",
		"zoneName": "Western Area Power Administration - Rocky Mountain Region"
	},
	"US-NW-WAUW": {
		"countryName": "USA",
		"displayName": "WAPA Upper Great Plains",
		"zoneName": "Western Area Power Administration - Upper Great Plains West"
	},
	"US-NY-NYIS": {
		"countryName": "USA",
		"zoneName": "New York ISO"
	},
	"US-SE-SEPA": {
		"countryName": "USA",
		"zoneName": "Southeastern Power Administration"
	},
	"US-SE-SOCO": {
		"countryName": "USA",
		"zoneName": "Southern Company Services"
	},
	"US-SW-AZPS": {
		"countryName": "USA",
		"zoneName": "Arizona Public Service Company"
	},
	"US-SW-EPE": {
		"countryName": "USA",
		"zoneName": "El Paso Electric Company"
	},
	"US-SW-PNM": {
		"countryName": "USA",
		"zoneName": "Public Service Company of New Mexico"
	},
	"US-SW-SRP": {
		"countryName": "USA",
		"zoneName": "Salt River Project"
	},
	"US-SW-TEPC": {
		"countryName": "USA",
		"zoneName": "Tucson Electric Power Company"
	},
	"US-SW-WALC": {
		"countryName": "USA",
		"displayName": "WAPA Desert Southwest",
		"zoneName": "Western Area Power Administration - Desert Southwest Region"
	},
	"US-TEN-TVA": {
		"countryName": "USA",
		"displayName": "TVA",
		"zoneName": "Tennessee Valley Authority"
	},
	"US-TEX-ERCO": {
		"countryName": "USA",
		"displayName": "ERCOT",
		"zoneName": "Electric Reliability Council of Texas"
	},
	"UY": {
		"zoneName": "Uruguay"
	},
	"UZ": {
		"zoneName": "Uzbekistan"
	},
	"VC": {
		"zoneName": "Saint Vincent and the Grenadines"
	},
	"VE": {
		"zoneName": "Venezuela"
	},
	"VI": {
		"countryName": "USA",
		"zoneName": "Virgin Islands"
	},
	"VN": {
		"zoneName": "Vietnam"
	},
	"VU": {
		"zoneName": "Vanuatu"
	},
	"WS": {
		"zoneName": "Samoa"
	},
	"XK": {
		"zoneName": "Kosovo"
	},
	"XX": {
		"zoneName": "Northern Cyprus"
	},
	"YE": {
		"zoneName": "Yemen"
	},
	"YT": {
		"zoneName": "Mayotte"
	},
	"ZA": {
		"zoneName": "South Africa"
	},
	"ZM": {
		"zoneName": "Zambia"
	},
	"ZW": {
		"zoneName": "Zimbabwe"
	}
}`))
				}))
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			server := tt.mockServer()
			defer server.Close()

			client := electricitymaps.NewElectricityMapsClient("test-api-key", electricitymaps.WithBaseUrl(server.URL))
			_, err := client.GetZones(ctx, false)

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetPowerBreakdownLatest(t *testing.T) {
	tests := []struct {
		name       string
		zone       string
		apiKey     string
		mockServer func() *httptest.Server
		expectErr  bool
	}{
		{
			name:   "Valid response",
			zone:   "US-NW-SCL",
			apiKey: "test-api-key",
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{
						"zone": "US-NW-SCL",
						"datetime": "2025-06-01T12:00:00Z",
						"powerConsumptionBreakdown": {"solar": 100, "wind": 200},
						"powerProductionBreakdown": {"solar": 150, "wind": 250}
					}`))
				}))
			},
			expectErr: false,
		},
		{
			name:   "Error response",
			zone:   "US-NW-SCL",
			apiKey: "test-api-key",
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			server := tt.mockServer()
			defer server.Close()

			client := electricitymaps.NewElectricityMapsClient(tt.apiKey, electricitymaps.WithBaseUrl(server.URL))
			_, err := client.GetPowerBreakdownLatest(ctx, tt.zone)

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
