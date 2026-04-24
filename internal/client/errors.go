package client

func ErrNotFound() error     { return errNotFound }
func ErrForbidden() error    { return errForbidden }
func ErrUnauthorized() error { return errUnauthorized }
