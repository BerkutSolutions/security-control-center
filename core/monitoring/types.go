package monitoring

import "strings"

const (
	TypeHTTP          = "http"
	TypeTCP           = "tcp"
	TypePing          = "ping"
	TypeHTTPKeyword   = "http_keyword"
	TypeHTTPJSON      = "http_json"
	TypeGRPCKeyword   = "grpc_keyword"
	TypeDNS           = "dns"
	TypeDocker        = "docker"
	TypePush          = "push"
	TypeSteam         = "steam"
	TypeGameDig       = "gamedig"
	TypeMQTT          = "mqtt"
	TypeKafkaProducer = "kafka_producer"
	TypeMSSQL         = "mssql"
	TypePostgres      = "postgres"
	TypeMySQL         = "mysql"
	TypeMongoDB       = "mongodb"
	TypeRadius        = "radius"
	TypeRedis         = "redis"
	TypeTailscalePing = "tailscale_ping"
)

func NormalizeType(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func IsSupportedType(raw string) bool {
	switch NormalizeType(raw) {
	case TypeHTTP, TypeTCP, TypePing, TypeHTTPKeyword, TypeHTTPJSON, TypeGRPCKeyword, TypeDNS,
		TypeDocker, TypePush, TypeSteam, TypeGameDig, TypeMQTT, TypeKafkaProducer, TypeMSSQL,
		TypePostgres, TypeMySQL, TypeMongoDB, TypeRadius, TypeRedis, TypeTailscalePing:
		return true
	default:
		return false
	}
}

func IsHTTPType(raw string) bool {
	switch NormalizeType(raw) {
	case TypeHTTP, TypeHTTPKeyword, TypeHTTPJSON:
		return true
	default:
		return false
	}
}

func TypeUsesURL(raw string) bool {
	switch NormalizeType(raw) {
	case TypeHTTP, TypeHTTPKeyword, TypeHTTPJSON, TypePostgres, TypeGRPCKeyword:
		return true
	default:
		return false
	}
}

func TypeUsesHostPort(raw string) bool {
	switch NormalizeType(raw) {
	case TypeTCP, TypePing, TypeDNS, TypeDocker, TypeSteam, TypeGameDig, TypeMQTT, TypeKafkaProducer,
		TypeMSSQL, TypeMySQL, TypeMongoDB, TypeRadius, TypeRedis, TypeTailscalePing:
		return true
	default:
		return false
	}
}

func TypeIsPassive(raw string) bool {
	return NormalizeType(raw) == TypePush
}

func TypeSupportsTLSMetadata(raw string) bool {
	switch NormalizeType(raw) {
	case TypeHTTP, TypeHTTPKeyword, TypeHTTPJSON, TypeGRPCKeyword:
		return true
	default:
		return false
	}
}

func DefaultPortForType(raw string) int {
	switch NormalizeType(raw) {
	case TypeRedis:
		return 6379
	case TypePostgres:
		return 5432
	case TypeMySQL:
		return 3306
	case TypeMSSQL:
		return 1433
	case TypeMongoDB:
		return 27017
	case TypeKafkaProducer:
		return 9092
	case TypeMQTT:
		return 1883
	case TypeRadius:
		return 1812
	case TypeDocker:
		return 2375
	case TypeSteam:
		return 27015
	case TypeGameDig:
		return 27015
	default:
		return 0
	}
}
