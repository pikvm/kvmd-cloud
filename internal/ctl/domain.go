package ctl

type ApplicationStatusResponse struct {
	PingerField string `json:"pinger"`
}

type CertbotDomainName struct {
	DomainName string `json:"domainName"`
	TXT        string `json:"txt"`
}

type CertbotResponse struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}
