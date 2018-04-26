package zkstore

// helpful for table tests for validations
func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
