package membrane

type ExecutionType int

const (
	ExecutionType_Service ExecutionType = iota
	ExecutionType_Job
)

func ExecutionTypeFromString(str string) ExecutionType {
	switch str {
	case "service":
		return ExecutionType_Service
	case "job":
		return ExecutionType_Job
	default:
		return ExecutionType_Service
	}
}
