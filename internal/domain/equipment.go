package domain

import "time"

// Equipment 设备：归属一个站点，属于一个设备族。
type Equipment struct {
	ID        string          `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	Family    string          `json:"family"`     // 设备族，配方按族适配
	StationID string          `json:"station_id"` // 设备服务的站点
	Status    EquipmentStatus `json:"status"`
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
}

// Chamber 设备腔体：具备能力标签，运行按腔体执行。
type Chamber struct {
	ID          string    `json:"id"`
	EquipmentID string    `json:"equipment_id"`
	Code        string    `json:"code"`
	Capability  string    `json:"capability"` // 腔体能力标签，须覆盖站点要求
	Status      string    `json:"status"`     // ACTIVE / DOWN
	CreatedAt   time.Time `json:"created_at"`
}

// Qualification 设备资质窗口：设备（或腔体）在某站点上的有效资质区间。
type Qualification struct {
	ID          string     `json:"id"`
	EquipmentID string     `json:"equipment_id"`
	ChamberID   string     `json:"chamber_id,omitempty"` // 空表示整台设备资质
	StationID   string     `json:"station_id"`
	ValidFrom   time.Time  `json:"valid_from"`
	ValidTo     time.Time  `json:"valid_to"`
	Status      QualStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Covers 判断资质窗口是否完整覆盖 [start, end]。
func (q Qualification) Covers(start, end time.Time) bool {
	if q.Status != QualActive {
		return false
	}
	return !q.ValidFrom.After(start) && q.ValidTo.After(end)
}

// Validate 校验设备字段。
func (e *Equipment) Validate() error {
	var fields []FieldError
	if e.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "设备编码不能为空"})
	}
	if e.Family == "" {
		fields = append(fields, FieldError{Field: "family", Message: "设备族不能为空"})
	}
	if e.StationID == "" {
		fields = append(fields, FieldError{Field: "station_id", Message: "设备站点不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}

// Validate 校验资质窗口字段。
func (q *Qualification) Validate() error {
	var fields []FieldError
	if q.EquipmentID == "" {
		fields = append(fields, FieldError{Field: "equipment_id", Message: "设备不能为空"})
	}
	if q.StationID == "" {
		fields = append(fields, FieldError{Field: "station_id", Message: "站点不能为空"})
	}
	if !q.ValidTo.After(q.ValidFrom) {
		fields = append(fields, FieldError{Field: "valid_to", Message: "资质截止时间必须晚于起始时间"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
