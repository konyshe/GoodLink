package config

var (
	m_version string
)

func SetVersion(v string) {
	m_version = v
}

func GetVersion() string {
	return m_version
}
