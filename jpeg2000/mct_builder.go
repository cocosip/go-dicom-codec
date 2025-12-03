package jpeg2000

type MCTBindingBuilder struct {
    b MCTBindingParams
}

func NewMCTBinding() *MCTBindingBuilder { return &MCTBindingBuilder{ b: MCTBindingParams{} } }
func (x *MCTBindingBuilder) Assoc(t uint8) *MCTBindingBuilder { x.b.AssocType = t; return x }
func (x *MCTBindingBuilder) Components(ids []uint16) *MCTBindingBuilder { x.b.ComponentIDs = ids; return x }
func (x *MCTBindingBuilder) Matrix(m [][]float64) *MCTBindingBuilder { x.b.Matrix = m; return x }
func (x *MCTBindingBuilder) Inverse(m [][]float64) *MCTBindingBuilder { x.b.Inverse = m; return x }
func (x *MCTBindingBuilder) Offsets(o []int32) *MCTBindingBuilder { x.b.Offsets = o; return x }
func (x *MCTBindingBuilder) ElementType(t uint8) *MCTBindingBuilder { x.b.ElementType = t; return x }
func (x *MCTBindingBuilder) MCOPrecision(p uint8) *MCTBindingBuilder { x.b.MCOPrecision = p; return x }
func (x *MCTBindingBuilder) NormScale(s float64) *MCTBindingBuilder { x.b.MCONormScale = s; return x }
func (x *MCTBindingBuilder) RecordOrder(order []uint8) *MCTBindingBuilder { x.b.MCTRecordOrder = order; return x }
func (x *MCTBindingBuilder) Build() MCTBindingParams { return x.b }

