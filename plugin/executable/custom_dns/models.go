package custom_dns

type RecordA struct {
	ID       int            `gorm:"primaryKey" json:"-"`
	Hostname string         `gorm:"index;size:253"`
	Value    []RecordAValue `gorm:"foreignKey:RecordRefer" json:"-"`
	TTL      uint
}

type RecordAValue struct {
	ID          int `gorm:"primaryKey,autoIncrement"`
	RecordRefer int `gorm:"index"`
	IPAddr      uint32
}

type RecordAAAA struct {
	ID       int               `gorm:"primaryKey" json:"-"`
	Hostname string            `gorm:"index;size:253"`
	Value    []RecordAAAAValue `gorm:"foreignKey:RecordRefer" json:"-"`
	TTL      uint
}

type RecordAAAAValue struct {
	ID          int `gorm:"primaryKey,autoIncrement"`
	RecordRefer int `gorm:"index"`
	IPAddrHi    int64
	IPAddrLo    int64
}

type RecordTXT struct {
	ID       int              `gorm:"index" json:"-"`
	Hostname string           `gorm:"size:253"`
	Value    []RecordTXTValue `gorm:"foreignKey:RecordRefer" json:"-"`
	TTL      uint
}

type RecordTXTValue struct {
	ID          int `gorm:"primaryKey,autoIncrement"`
	RecordRefer int `gorm:"index"`
	TXT         string
}
