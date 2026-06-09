package model

type Credentials struct {
	Token        string `json:"token"`
	Username     string `json:"username"`
	RefreshToken string `json:"refreshToken"`
	EmailHash    string `json:"emailHash"`
}
