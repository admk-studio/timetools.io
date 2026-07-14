package tz

// alias maps a query people actually type to a canonical zone. City
// aliases carry their own coordinates so sunrise/sunset comes out right
// for the city that was asked about, not the zone's principal city —
// Miami and New York share a zone but not a sunrise.
type alias struct {
	Zone      string
	Display   string // empty means "use the zone's own city name"
	Lat, Lon  float64
	HasCoords bool
}

func city(zone, display string, lat, lon float64) alias {
	return alias{Zone: zone, Display: display, Lat: lat, Lon: lon, HasCoords: true}
}

// abbr is for abbreviations like "nyc" or "pst" where the zone's own city
// is the right thing to show.
func abbr(zone string) alias {
	return alias{Zone: zone}
}

// Keys are matched after normalization, so "new delhi", "New_Delhi" and
// "newdelhi" are the same entry. Keep the list alphabetized within each
// group; PRs adding a city should include coordinates.
var aliases = map[string]alias{
	// UTC and friends.
	"utc":  {Zone: "UTC", Display: "UTC"},
	"gmt":  {Zone: "UTC", Display: "UTC"},
	"zulu": {Zone: "UTC", Display: "UTC"},

	// North America.
	"atlanta":        city("America/New_York", "Atlanta", 33.75, -84.39),
	"austin":         city("America/Chicago", "Austin", 30.27, -97.74),
	"boston":         city("America/New_York", "Boston", 42.36, -71.06),
	"calgary":        city("America/Edmonton", "Calgary", 51.05, -114.07),
	"dallas":         city("America/Chicago", "Dallas", 32.78, -96.80),
	"dc":             city("America/New_York", "Washington DC", 38.91, -77.04),
	"el paso":        city("America/Denver", "El Paso", 31.76, -106.49),
	"guadalajara":    city("America/Mexico_City", "Guadalajara", 20.67, -103.35),
	"houston":        city("America/Chicago", "Houston", 29.76, -95.37),
	"kansas city":    city("America/Chicago", "Kansas City", 39.10, -94.58),
	"la":             abbr("America/Los_Angeles"),
	"las vegas":      city("America/Los_Angeles", "Las Vegas", 36.17, -115.14),
	"memphis":        city("America/Chicago", "Memphis", 35.15, -90.05),
	"miami":          city("America/New_York", "Miami", 25.76, -80.19),
	"milwaukee":      city("America/Chicago", "Milwaukee", 43.04, -87.91),
	"minneapolis":    city("America/Chicago", "Minneapolis", 44.98, -93.27),
	"montreal":       city("America/Toronto", "Montreal", 45.50, -73.57),
	"nashville":      city("America/Chicago", "Nashville", 36.16, -86.78),
	"new orleans":    city("America/Chicago", "New Orleans", 29.95, -90.07),
	"new york city":  abbr("America/New_York"),
	"nyc":            abbr("America/New_York"),
	"ottawa":         city("America/Toronto", "Ottawa", 45.42, -75.70),
	"philadelphia":   city("America/New_York", "Philadelphia", 39.95, -75.17),
	"philly":         city("America/New_York", "Philadelphia", 39.95, -75.17),
	"pittsburgh":     city("America/New_York", "Pittsburgh", 40.44, -79.98),
	"portland":       city("America/Los_Angeles", "Portland", 45.52, -122.68),
	"quebec":         city("America/Toronto", "Quebec City", 46.81, -71.21),
	"sacramento":     city("America/Los_Angeles", "Sacramento", 38.58, -121.49),
	"salt lake city": city("America/Denver", "Salt Lake City", 40.76, -111.89),
	"san antonio":    city("America/Chicago", "San Antonio", 29.42, -98.49),
	"san diego":      city("America/Los_Angeles", "San Diego", 32.72, -117.16),
	"san francisco":  city("America/Los_Angeles", "San Francisco", 37.77, -122.42),
	"san jose":       city("America/Los_Angeles", "San Jose", 37.34, -121.89),
	"seattle":        city("America/Los_Angeles", "Seattle", 47.61, -122.33),
	"sf":             city("America/Los_Angeles", "San Francisco", 37.77, -122.42),
	"st louis":       city("America/Chicago", "St Louis", 38.63, -90.20),
	"tucson":         city("America/Phoenix", "Tucson", 32.22, -110.97),
	"vegas":          city("America/Los_Angeles", "Las Vegas", 36.17, -115.14),
	"washington":     city("America/New_York", "Washington DC", 38.91, -77.04),
	"washington dc":  city("America/New_York", "Washington DC", 38.91, -77.04),

	// Latin America.
	"brasilia":       city("America/Sao_Paulo", "Brasília", -15.79, -47.88),
	"medellin":       city("America/Bogota", "Medellín", 6.25, -75.56),
	"quito":          city("America/Guayaquil", "Quito", -0.18, -78.47),
	"rio":            city("America/Sao_Paulo", "Rio de Janeiro", -22.91, -43.17),
	"rio de janeiro": city("America/Sao_Paulo", "Rio de Janeiro", -22.91, -43.17),

	// Europe. Several capitals stopped being canonical zones when tzdb
	// merged duplicates (2022b and later), so they live here instead.
	"amsterdam":     city("Europe/Brussels", "Amsterdam", 52.37, 4.90),
	"barcelona":     city("Europe/Madrid", "Barcelona", 41.39, 2.17),
	"birmingham":    city("Europe/London", "Birmingham", 52.48, -1.90),
	"cologne":       city("Europe/Berlin", "Cologne", 50.94, 6.96),
	"copenhagen":    city("Europe/Berlin", "Copenhagen", 55.68, 12.57),
	"edinburgh":     city("Europe/London", "Edinburgh", 55.95, -3.19),
	"florence":      city("Europe/Rome", "Florence", 43.77, 11.26),
	"frankfurt":     city("Europe/Berlin", "Frankfurt", 50.11, 8.68),
	"geneva":        city("Europe/Zurich", "Geneva", 46.20, 6.14),
	"glasgow":       city("Europe/London", "Glasgow", 55.86, -4.25),
	"hamburg":       city("Europe/Berlin", "Hamburg", 53.55, 9.99),
	"krakow":        city("Europe/Warsaw", "Kraków", 50.06, 19.94),
	"kiev":          city("Europe/Kyiv", "Kyiv", 50.45, 30.52),
	"luxembourg":    city("Europe/Brussels", "Luxembourg", 49.61, 6.13),
	"lyon":          city("Europe/Paris", "Lyon", 45.76, 4.84),
	"manchester":    city("Europe/London", "Manchester", 53.48, -2.24),
	"marseille":     city("Europe/Paris", "Marseille", 43.30, 5.37),
	"milan":         city("Europe/Rome", "Milan", 45.46, 9.19),
	"monaco":        city("Europe/Paris", "Monaco", 43.73, 7.42),
	"munich":        city("Europe/Berlin", "Munich", 48.14, 11.58),
	"naples":        city("Europe/Rome", "Naples", 40.85, 14.27),
	"nice":          city("Europe/Paris", "Nice", 43.70, 7.27),
	"oslo":          city("Europe/Berlin", "Oslo", 59.91, 10.75),
	"porto":         city("Europe/Lisbon", "Porto", 41.15, -8.61),
	"reykjavik":     city("Africa/Abidjan", "Reykjavík", 64.15, -21.94),
	"seville":       city("Europe/Madrid", "Seville", 37.39, -5.98),
	"st petersburg": city("Europe/Moscow", "St Petersburg", 59.94, 30.31),
	"stockholm":     city("Europe/Berlin", "Stockholm", 59.33, 18.07),
	"turin":         city("Europe/Rome", "Turin", 45.07, 7.69),
	"valencia":      city("Europe/Madrid", "Valencia", 39.47, -0.38),
	"venice":        city("Europe/Rome", "Venice", 45.44, 12.34),

	// Africa & Middle East.
	"abu dhabi":     city("Asia/Dubai", "Abu Dhabi", 24.45, 54.38),
	"abuja":         city("Africa/Lagos", "Abuja", 9.06, 7.49),
	"accra":         city("Africa/Abidjan", "Accra", 5.60, -0.19),
	"addis ababa":   city("Africa/Nairobi", "Addis Ababa", 9.03, 38.74),
	"ankara":        city("Europe/Istanbul", "Ankara", 39.93, 32.86),
	"cape town":     city("Africa/Johannesburg", "Cape Town", -33.92, 18.42),
	"dakar":         city("Africa/Abidjan", "Dakar", 14.72, -17.47),
	"dar es salaam": city("Africa/Nairobi", "Dar es Salaam", -6.79, 39.21),
	"doha":          city("Asia/Qatar", "Doha", 25.29, 51.53),
	"durban":        city("Africa/Johannesburg", "Durban", -29.86, 31.03),
	"kampala":       city("Africa/Nairobi", "Kampala", 0.35, 32.58),
	"kinshasa":      city("Africa/Lagos", "Kinshasa", -4.32, 15.31),
	"kuwait":        city("Asia/Riyadh", "Kuwait City", 29.38, 47.99),
	"kuwait city":   city("Asia/Riyadh", "Kuwait City", 29.38, 47.99),
	"muscat":        city("Asia/Dubai", "Muscat", 23.59, 58.41),
	"tel aviv":      city("Asia/Jerusalem", "Tel Aviv", 32.09, 34.78),

	// Asia.
	"bangalore":    city("Asia/Kolkata", "Bengaluru", 12.97, 77.59),
	"beijing":      city("Asia/Shanghai", "Beijing", 39.90, 116.41),
	"bengaluru":    city("Asia/Kolkata", "Bengaluru", 12.97, 77.59),
	"bombay":       city("Asia/Kolkata", "Mumbai", 19.08, 72.88),
	"busan":        city("Asia/Seoul", "Busan", 35.18, 129.08),
	"chennai":      city("Asia/Kolkata", "Chennai", 13.08, 80.27),
	"delhi":        city("Asia/Kolkata", "New Delhi", 28.61, 77.21),
	"guangzhou":    city("Asia/Shanghai", "Guangzhou", 23.13, 113.26),
	"hanoi":        city("Asia/Ho_Chi_Minh", "Hanoi", 21.03, 105.85),
	"hk":           abbr("Asia/Hong_Kong"),
	"hyderabad":    city("Asia/Kolkata", "Hyderabad", 17.39, 78.49),
	"islamabad":    city("Asia/Karachi", "Islamabad", 33.69, 73.06),
	"kl":           city("Asia/Singapore", "Kuala Lumpur", 3.14, 101.69),
	"kuala lumpur": city("Asia/Singapore", "Kuala Lumpur", 3.14, 101.69),
	"kyoto":        city("Asia/Tokyo", "Kyoto", 35.01, 135.77),
	"lahore":       city("Asia/Karachi", "Lahore", 31.55, 74.34),
	"mumbai":       city("Asia/Kolkata", "Mumbai", 19.08, 72.88),
	"nagoya":       city("Asia/Tokyo", "Nagoya", 35.18, 136.91),
	"new delhi":    city("Asia/Kolkata", "New Delhi", 28.61, 77.21),
	"osaka":        city("Asia/Tokyo", "Osaka", 34.69, 135.50),
	"pune":         city("Asia/Kolkata", "Pune", 18.52, 73.86),
	"saigon":       abbr("Asia/Ho_Chi_Minh"),
	"sg":           abbr("Asia/Singapore"),
	"shenzhen":     city("Asia/Shanghai", "Shenzhen", 22.54, 114.06),

	// Oceania.
	"canberra":     city("Australia/Sydney", "Canberra", -35.28, 149.13),
	"christchurch": city("Pacific/Auckland", "Christchurch", -43.53, 172.64),
	"wellington":   city("Pacific/Auckland", "Wellington", -41.29, 174.78),

	// Countries where the answer is unambiguous enough to be useful.
	"china":       abbr("Asia/Shanghai"),
	"france":      abbr("Europe/Paris"),
	"germany":     abbr("Europe/Berlin"),
	"india":       city("Asia/Kolkata", "India", 28.61, 77.21),
	"italy":       abbr("Europe/Rome"),
	"japan":       abbr("Asia/Tokyo"),
	"netherlands": city("Europe/Brussels", "Amsterdam", 52.37, 4.90),
	"spain":       abbr("Europe/Madrid"),
	"uk":          abbr("Europe/London"),

	// Time zone abbreviations. These are ambiguous by nature (CST is
	// Chicago and China, IST is India, Israel and Ireland) so we map each
	// to its most common meaning and show the city we picked.
	"acst":     abbr("Australia/Adelaide"),
	"aedt":     abbr("Australia/Sydney"),
	"aest":     abbr("Australia/Sydney"),
	"awst":     abbr("Australia/Perth"),
	"bst":      abbr("Europe/London"),
	"cdt":      abbr("America/Chicago"),
	"cet":      abbr("Europe/Berlin"),
	"central":  abbr("America/Chicago"),
	"cst":      abbr("America/Chicago"),
	"eastern":  abbr("America/New_York"),
	"edt":      abbr("America/New_York"),
	"eet":      abbr("Europe/Athens"),
	"est":      abbr("America/New_York"),
	"gst":      abbr("Asia/Dubai"),
	"hkt":      abbr("Asia/Hong_Kong"),
	"ist":      city("Asia/Kolkata", "India", 28.61, 77.21),
	"jst":      abbr("Asia/Tokyo"),
	"kst":      abbr("Asia/Seoul"),
	"mdt":      abbr("America/Denver"),
	"mountain": abbr("America/Denver"),
	"msk":      abbr("Europe/Moscow"),
	"mst":      abbr("America/Denver"),
	"nzdt":     abbr("Pacific/Auckland"),
	"nzst":     abbr("Pacific/Auckland"),
	"pacific":  abbr("America/Los_Angeles"),
	"pdt":      abbr("America/Los_Angeles"),
	"pht":      abbr("Asia/Manila"),
	"pst":      abbr("America/Los_Angeles"),
	"sgt":      abbr("Asia/Singapore"),
	"wet":      abbr("Europe/Lisbon"),
	"wib":      abbr("Asia/Jakarta"),
}
