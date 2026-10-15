package shop

import "fmt"

type Status int

const (
	Pending Status = iota
	Paid
	Shipped
	Cancelled
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "pending"
	case Paid:
		return "paid"
	case Shipped:
		return "shipped"
	case Cancelled:
		return "cancelled"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// ParseStatus reads a status back from the database. Cancelled came later, and nobody
// added it here.
func ParseStatus(s string) (Status, error) {
	switch s {
	case "pending":
		return Pending, nil
	case "paid":
		return Paid, nil
	case "shipped":
		return Shipped, nil
	}
	return 0, fmt.Errorf("unknown status %q", s)
}
