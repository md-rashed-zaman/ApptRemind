package entitlements

// Limits represents the entitlements derived from a subscription tier.
// Keep this small and stable: other services may rely on these limits to enforce behavior.
type Limits struct {
	Tier                  string `json:"tier"`
	MaxStaff              int32  `json:"max_staff"`
	MaxServices           int32  `json:"max_services"`
	MaxMonthlyAppointments int32  `json:"max_monthly_appointments"`
}

func LimitsForTier(tier string) Limits {
	switch tier {
	case "starter":
		return Limits{
			Tier:                  "starter",
			MaxStaff:              1,
			MaxServices:           3,
			MaxMonthlyAppointments: 1,
		}
	case "pro":
		return Limits{
			Tier:                  "pro",
			MaxStaff:              25,
			MaxServices:           100,
			MaxMonthlyAppointments: 2000,
		}
	default:
		return Limits{
			Tier:                  "free",
			MaxStaff:              3,
			MaxServices:           10,
			MaxMonthlyAppointments: 200,
		}
	}
}

