package gatecontrol

type permissionRequest struct {
	Location     string `json:"location"`
	Loadingplace int64  `json:"loadingplaceId"`
	Token        string `json:"token"`
}

type message struct {
	MessageCode string `json:"messageCode"`
}

type permissionResponse struct {
	Permitted bool     `json:"permitted"`
	Message   *message `json:"message"`
}

type validatePurpose int

const (
	validateEntry validatePurpose = iota
	validateExit
)

func (purpose validatePurpose) Type() string {
	names := [...]string{
		"net.contargo.terminalpermission.validate.token.entry",
		"net.contargo.terminalpermission.validate.token.exit",
	}
	return names[purpose]
}

func (purpose validatePurpose) Version() string {
	return "v2"
}

func (purpose validatePurpose) Rk() string {
	names := [...]string{
		"terminalpermission.validate.entry",
		"terminalpermission.validate.exit",
	}
	return names[purpose]
}

type usePurpose int

const (
	useEntry usePurpose = iota
	useExit
)

func (purpose usePurpose) Type() string {
	names := [...]string{
		"net.contargo.terminalpermission.use.token.entry",
		"net.contargo.terminalpermission.use.token.exit",
	}
	return names[purpose]
}

func (purpose usePurpose) Version() string {
	return "v1"
}

func (purpose usePurpose) Rk() string {
	names := [...]string{
		"terminalpermission.use",
		"terminalpermission.use",
	}
	return names[purpose]
}
