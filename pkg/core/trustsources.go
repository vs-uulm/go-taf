package core

import "strings"

/*
TrustSource specifies the type of trust source.
*/
type TrustSource uint16

/*
EvidenceType specifies the type of evidence. Each evidence is prefixed by the trust source type it belongs to.
*/
type EvidenceType uint16

const (
	/*
		Unknown trust source, should not be used.
	*/
	NONE TrustSource = iota
	/*
		AIV
	*/
	AIV
	/*
		MBD
	*/
	MBD
	/*
		TCH
	*/
	TCH
	/*
	   NTM
	*/
	NTM
)

const (
	UNKNOWN EvidenceType = iota
	AIV_SECURE_BOOT
	AIV_SECURE_OTA
	AIV_ACCESS_CONTROL
	AIV_APPLICATION_ISOLATION
	AIV_CONTROL_FLOW_INTEGRITY
	AIV_CONFIGURATION_INTEGRITY_VERIFICATION
	MBD_MISBEHAVIOR_REPORT
	MBD_RELATIVE_POSITION_ERROR_X
	MBD_RELATIVE_POSITION_ERROR_Y
	MBD_SENDER_SPEED_ERROR_X
	MBD_SENDER_SPEED_ERROR_Y
	MBD_SENDER_ACCELERATION_ERROR_X
	MBD_SENDER_ACCELERATION_ERROR_Y
	MBD_DISTANCE_TO_ROAD_EDGE_ERROR
	MBD_RECEIVER_TIME_ERROR
	MBD_SENDER_HEADING_ERROR_SIN
	MBD_SENDER_HEADING_ERROR_COS
	TCH_SECURE_BOOT
	TCH_SECURE_OTA
	TCH_ACCESS_CONTROL
	TCH_APPLICATION_ISOLATION
	TCH_CONTROL_FLOW_INTEGRITY
	TCH_CONFIGURATION_INTEGRITY_VERIFICATION
	NTM_REMOTE_OPINION
)

func (e EvidenceType) String() string {
	switch e {
	case UNKNOWN:
		return "UNKNOWN"
	case AIV_SECURE_BOOT:
		return "SECURE_BOOT"
	case AIV_SECURE_OTA:
		return "SECURE_OTA"
	case AIV_ACCESS_CONTROL:
		return "ACCESS_CONTROL"
	case AIV_APPLICATION_ISOLATION:
		return "APPLICATION_ISOLATION"
	case AIV_CONTROL_FLOW_INTEGRITY:
		return "CONTROL_FLOW_INTEGRITY"
	case AIV_CONFIGURATION_INTEGRITY_VERIFICATION:
		return "CONFIGURATION_INTEGRITY_VERIFICATION"
	case MBD_MISBEHAVIOR_REPORT:
		return "MISBEHAVIOR_REPORT"
	case MBD_RELATIVE_POSITION_ERROR_X:
		return "RELATIVE_POSITION_ERROR_X"
	case MBD_RELATIVE_POSITION_ERROR_Y:
		return "RELATIVE_POSITION_ERROR_Y"
	case MBD_SENDER_SPEED_ERROR_X:
		return "SENDER_SPEED_ERROR_X"
	case MBD_SENDER_SPEED_ERROR_Y:
		return "SENDER_SPEED_ERROR_Y"
	case MBD_SENDER_ACCELERATION_ERROR_X:
		return "SENDER_ACCELERATION_ERROR_X"
	case MBD_SENDER_ACCELERATION_ERROR_Y:
		return "SENDER_ACCELERATION_ERROR_Y"
	case MBD_DISTANCE_TO_ROAD_EDGE_ERROR:
		return "DISTANCE_TO_ROAD_EDGE_ERROR"
	case MBD_RECEIVER_TIME_ERROR:
		return "RECEIVER_TIME_ERROR"
	case MBD_SENDER_HEADING_ERROR_SIN:
		return "SENDER_HEADING_ERROR_SIN"
	case MBD_SENDER_HEADING_ERROR_COS:
		return "SENDER_HEADING_ERROR_COS"
	case TCH_SECURE_BOOT:
		return "SECURE_BOOT"
	case TCH_SECURE_OTA:
		return "SECURE_OTA"
	case TCH_ACCESS_CONTROL:
		return "ACCESS_CONTROL"
	case TCH_APPLICATION_ISOLATION:
		return "APPLICATION_ISOLATION"
	case TCH_CONTROL_FLOW_INTEGRITY:
		return "CONTROL_FLOW_INTEGRITY"
	case TCH_CONFIGURATION_INTEGRITY_VERIFICATION:
		return "CONFIGURATION_INTEGRITY_VERIFICATION"
	case NTM_REMOTE_OPINION:
		return "REMOTE_OPINION"
	default:
		return "UNKNOWN_EVIDENCE"
	}
}

func (s TrustSource) String() string {
	switch s {
	case NONE:
		return "NONE"
	case AIV:
		return "AIV"
	case MBD:
		return "MBD"
	case TCH:
		return "TCH"
	case NTM:
		return "NTM"
	default:
		return "UNKNOWN_SOURCE"
	}
}

func (e EvidenceType) Source() TrustSource {
	switch e {
	case UNKNOWN:
		return NONE
	case AIV_SECURE_BOOT:
		return AIV
	case AIV_SECURE_OTA:
		return AIV
	case AIV_ACCESS_CONTROL:
		return AIV
	case AIV_APPLICATION_ISOLATION:
		return AIV
	case AIV_CONTROL_FLOW_INTEGRITY:
		return AIV
	case AIV_CONFIGURATION_INTEGRITY_VERIFICATION:
		return AIV
	case MBD_MISBEHAVIOR_REPORT:
		return MBD
	case MBD_RELATIVE_POSITION_ERROR_X:
		return MBD
	case MBD_RELATIVE_POSITION_ERROR_Y:
		return MBD
	case MBD_SENDER_SPEED_ERROR_X:
		return MBD
	case MBD_SENDER_SPEED_ERROR_Y:
		return MBD
	case MBD_SENDER_ACCELERATION_ERROR_X:
		return MBD
	case MBD_SENDER_ACCELERATION_ERROR_Y:
		return MBD
	case MBD_DISTANCE_TO_ROAD_EDGE_ERROR:
		return MBD
	case MBD_RECEIVER_TIME_ERROR:
		return MBD
	case MBD_SENDER_HEADING_ERROR_SIN:
		return MBD
	case MBD_SENDER_HEADING_ERROR_COS:
		return MBD
	case TCH_SECURE_BOOT:
		return TCH
	case TCH_SECURE_OTA:
		return TCH
	case TCH_ACCESS_CONTROL:
		return TCH
	case TCH_APPLICATION_ISOLATION:
		return TCH
	case TCH_CONTROL_FLOW_INTEGRITY:
		return TCH
	case TCH_CONFIGURATION_INTEGRITY_VERIFICATION:
		return TCH
	case NTM_REMOTE_OPINION:
		return NTM
	default:
		return NONE
	}
}

func EvidenceTypeBySourceAndName(ts TrustSource, name string) EvidenceType {
	switch ts {
	case AIV:
		switch {
		case strings.ToUpper(name) == AIV_SECURE_BOOT.String():
			return AIV_SECURE_BOOT
		case strings.ToUpper(name) == AIV_SECURE_OTA.String():
			return AIV_SECURE_OTA
		case strings.ToUpper(name) == AIV_ACCESS_CONTROL.String():
			return AIV_ACCESS_CONTROL
		case strings.ToUpper(name) == AIV_APPLICATION_ISOLATION.String():
			return AIV_APPLICATION_ISOLATION
		case strings.ToUpper(name) == AIV_CONTROL_FLOW_INTEGRITY.String():
			return AIV_CONTROL_FLOW_INTEGRITY
		case strings.ToUpper(name) == AIV_CONFIGURATION_INTEGRITY_VERIFICATION.String():
			return AIV_CONFIGURATION_INTEGRITY_VERIFICATION
		default:
			return UNKNOWN
		}
	case MBD:
		switch {
		case evidenceNameMatches(name, MBD_MISBEHAVIOR_REPORT):
			return MBD_MISBEHAVIOR_REPORT
		case evidenceNameMatches(name, MBD_RELATIVE_POSITION_ERROR_X):
			return MBD_RELATIVE_POSITION_ERROR_X
		case evidenceNameMatches(name, MBD_RELATIVE_POSITION_ERROR_Y):
			return MBD_RELATIVE_POSITION_ERROR_Y
		case evidenceNameMatches(name, MBD_SENDER_SPEED_ERROR_X):
			return MBD_SENDER_SPEED_ERROR_X
		case evidenceNameMatches(name, MBD_SENDER_SPEED_ERROR_Y):
			return MBD_SENDER_SPEED_ERROR_Y
		case evidenceNameMatches(name, MBD_SENDER_ACCELERATION_ERROR_X):
			return MBD_SENDER_ACCELERATION_ERROR_X
		case evidenceNameMatches(name, MBD_SENDER_ACCELERATION_ERROR_Y):
			return MBD_SENDER_ACCELERATION_ERROR_Y
		case evidenceNameMatches(name, MBD_DISTANCE_TO_ROAD_EDGE_ERROR):
			return MBD_DISTANCE_TO_ROAD_EDGE_ERROR
		case evidenceNameMatches(name, MBD_RECEIVER_TIME_ERROR):
			return MBD_RECEIVER_TIME_ERROR
		case evidenceNameMatches(name, MBD_SENDER_HEADING_ERROR_SIN):
			return MBD_SENDER_HEADING_ERROR_SIN
		case evidenceNameMatches(name, MBD_SENDER_HEADING_ERROR_COS):
			return MBD_SENDER_HEADING_ERROR_COS
		default:
			return UNKNOWN
		}
	case TCH:
		switch {
		case strings.ToUpper(name) == TCH_SECURE_BOOT.String():
			return TCH_SECURE_BOOT
		case strings.ToUpper(name) == TCH_SECURE_OTA.String():
			return TCH_SECURE_OTA
		case strings.ToUpper(name) == TCH_ACCESS_CONTROL.String():
			return TCH_ACCESS_CONTROL
		case strings.ToUpper(name) == TCH_APPLICATION_ISOLATION.String():
			return TCH_APPLICATION_ISOLATION
		case strings.ToUpper(name) == TCH_CONTROL_FLOW_INTEGRITY.String():
			return TCH_CONTROL_FLOW_INTEGRITY
		case strings.ToUpper(name) == TCH_CONFIGURATION_INTEGRITY_VERIFICATION.String():
			return TCH_CONFIGURATION_INTEGRITY_VERIFICATION
		default:
			return UNKNOWN
		}
	case NTM:
		switch {
		case strings.ToUpper(name) == NTM_REMOTE_OPINION.String():
			return NTM_REMOTE_OPINION
		default:
			return UNKNOWN
		}
	default:
		return UNKNOWN
	}
}

func evidenceNameMatches(name string, evidence EvidenceType) bool {
	upperName := strings.ToUpper(name)
	return upperName == evidence.String() || upperName == evidence.Source().String()+"_"+evidence.String()
}
