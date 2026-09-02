package creation

// ClientInput is the identity carried by a particulier or entreprise
// request. CompanyName and RCNumber are read only by the entreprise
// interactor; a restitution has no client at all.
type ClientInput struct {
	LastName    string
	FirstName   string
	BirthDate   string // yyyy-mm-dd, exactly as bound from the request JSON
	BirthPlace  string
	IDType      string
	IDNumber    string
	CompanyName string
	RCNumber    string
}
