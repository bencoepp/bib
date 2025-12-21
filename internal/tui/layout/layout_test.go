package layout

import (
	"testing"
)

func TestDirection_Constants(t *testing.T) {
	if Row != 0 {
		t.Errorf("expected Row = 0, got %d", Row)
	}
	if Column != 1 {
		t.Errorf("expected Column = 1, got %d", Column)
	}
}

func TestJustify_Constants(t *testing.T) {
	tests := []struct {
		justify Justify
		value   int
	}{
		{JustifyStart, 0},
		{JustifyCenter, 1},
		{JustifyEnd, 2},
		{JustifySpaceBetween, 3},
		{JustifySpaceAround, 4},
		{JustifySpaceEvenly, 5},
	}

	for _, tt := range tests {
		if int(tt.justify) != tt.value {
			t.Errorf("expected %d, got %d", tt.value, tt.justify)
		}
	}
}

func TestAlign_Constants(t *testing.T) {
	tests := []struct {
		align Align
		value int
	}{
		{AlignStart, 0},
		{AlignCenter, 1},
		{AlignEnd, 2},
		{AlignStretch, 3},
	}

	for _, tt := range tests {
		if int(tt.align) != tt.value {
			t.Errorf("expected %d, got %d", tt.value, tt.align)
		}
	}
}

func TestNewFlex(t *testing.T) {
	f := NewFlex()
	if f == nil {
		t.Fatal("expected non-nil Flex")
	}

	if f.direction != Row {
		t.Errorf("expected default direction Row, got %d", f.direction)
	}
	if f.justify != JustifyStart {
		t.Errorf("expected default justify JustifyStart, got %d", f.justify)
	}
	if f.align != AlignStart {
		t.Errorf("expected default align AlignStart, got %d", f.align)
	}
	if f.gap != 0 {
		t.Errorf("expected default gap 0, got %d", f.gap)
	}
	if f.wrap != false {
		t.Error("expected default wrap false")
	}
}

func TestFlex_Direction(t *testing.T) {
	f := NewFlex().Direction(Column)
	if f.direction != Column {
		t.Errorf("expected Column, got %d", f.direction)
	}
}

func TestFlex_Justify(t *testing.T) {
	f := NewFlex().Justify(JustifyCenter)
	if f.justify != JustifyCenter {
		t.Errorf("expected JustifyCenter, got %d", f.justify)
	}
}

func TestFlex_Align(t *testing.T) {
	f := NewFlex().Align(AlignStretch)
	if f.align != AlignStretch {
		t.Errorf("expected AlignStretch, got %d", f.align)
	}
}

func TestFlex_Gap(t *testing.T) {
	f := NewFlex().Gap(5)
	if f.gap != 5 {
		t.Errorf("expected gap 5, got %d", f.gap)
	}
}

func TestFlex_Wrap(t *testing.T) {
	f := NewFlex().Wrap(true)
	if !f.wrap {
		t.Error("expected wrap true")
	}
}

func TestFlex_Chaining(t *testing.T) {
	f := NewFlex().
		Direction(Column).
		Justify(JustifySpaceBetween).
		Align(AlignCenter).
		Gap(2).
		Wrap(true)

	if f.direction != Column {
		t.Error("direction not set correctly")
	}
	if f.justify != JustifySpaceBetween {
		t.Error("justify not set correctly")
	}
	if f.align != AlignCenter {
		t.Error("align not set correctly")
	}
	if f.gap != 2 {
		t.Error("gap not set correctly")
	}
	if !f.wrap {
		t.Error("wrap not set correctly")
	}
}

func TestBreakpoint_Constants(t *testing.T) {
	tests := []struct {
		bp    Breakpoint
		value int
	}{
		{BreakpointXS, 0},
		{BreakpointSM, 1},
		{BreakpointMD, 2},
		{BreakpointLG, 3},
		{BreakpointXL, 4},
		{BreakpointXXL, 5},
	}

	for _, tt := range tests {
		if int(tt.bp) != tt.value {
			t.Errorf("expected %d, got %d", tt.value, tt.bp)
		}
	}
}

func TestBreakpointThresholds(t *testing.T) {
	if BreakpointThresholds[BreakpointXS] != 30 {
		t.Errorf("expected XS threshold 30, got %d", BreakpointThresholds[BreakpointXS])
	}
	if BreakpointThresholds[BreakpointSM] != 50 {
		t.Errorf("expected SM threshold 50, got %d", BreakpointThresholds[BreakpointSM])
	}
	if BreakpointThresholds[BreakpointMD] != 80 {
		t.Errorf("expected MD threshold 80, got %d", BreakpointThresholds[BreakpointMD])
	}
	if BreakpointThresholds[BreakpointLG] != 120 {
		t.Errorf("expected LG threshold 120, got %d", BreakpointThresholds[BreakpointLG])
	}
	if BreakpointThresholds[BreakpointXL] != 180 {
		t.Errorf("expected XL threshold 180, got %d", BreakpointThresholds[BreakpointXL])
	}
	if BreakpointThresholds[BreakpointXXL] != 240 {
		t.Errorf("expected XXL threshold 240, got %d", BreakpointThresholds[BreakpointXXL])
	}
}

func TestGetBreakpoint(t *testing.T) {
	tests := []struct {
		width    int
		expected Breakpoint
	}{
		{10, BreakpointXS},
		{29, BreakpointXS},
		{30, BreakpointXS},
		{49, BreakpointXS},
		{50, BreakpointSM},
		{79, BreakpointSM},
		{80, BreakpointMD},
		{119, BreakpointMD},
		{120, BreakpointLG},
		{179, BreakpointLG},
		{180, BreakpointXL},
		{239, BreakpointXL},
		{240, BreakpointXXL},
		{300, BreakpointXXL},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetBreakpoint(tt.width)
			if got != tt.expected {
				t.Errorf("GetBreakpoint(%d) = %d, want %d", tt.width, got, tt.expected)
			}
		})
	}
}

func TestHeightBreakpoint_Constants(t *testing.T) {
	tests := []struct {
		bp    HeightBreakpoint
		value int
	}{
		{HeightBreakpointXS, 0},
		{HeightBreakpointSM, 1},
		{HeightBreakpointMD, 2},
		{HeightBreakpointLG, 3},
		{HeightBreakpointXL, 4},
	}

	for _, tt := range tests {
		if int(tt.bp) != tt.value {
			t.Errorf("expected %d, got %d", tt.value, tt.bp)
		}
	}
}

func TestHeightBreakpointThresholds(t *testing.T) {
	if HeightBreakpointThresholds[HeightBreakpointXS] != 0 {
		t.Errorf("expected XS threshold 0, got %d", HeightBreakpointThresholds[HeightBreakpointXS])
	}
	if HeightBreakpointThresholds[HeightBreakpointSM] != 15 {
		t.Errorf("expected SM threshold 15, got %d", HeightBreakpointThresholds[HeightBreakpointSM])
	}
	if HeightBreakpointThresholds[HeightBreakpointMD] != 24 {
		t.Errorf("expected MD threshold 24, got %d", HeightBreakpointThresholds[HeightBreakpointMD])
	}
	if HeightBreakpointThresholds[HeightBreakpointLG] != 40 {
		t.Errorf("expected LG threshold 40, got %d", HeightBreakpointThresholds[HeightBreakpointLG])
	}
	if HeightBreakpointThresholds[HeightBreakpointXL] != 60 {
		t.Errorf("expected XL threshold 60, got %d", HeightBreakpointThresholds[HeightBreakpointXL])
	}
}

func TestGetHeightBreakpoint(t *testing.T) {
	tests := []struct {
		height   int
		expected HeightBreakpoint
	}{
		{10, HeightBreakpointXS},
		{14, HeightBreakpointXS},
		{15, HeightBreakpointSM},
		{23, HeightBreakpointSM},
		{24, HeightBreakpointMD},
		{39, HeightBreakpointMD},
		{40, HeightBreakpointLG},
		{59, HeightBreakpointLG},
		{60, HeightBreakpointXL},
		{100, HeightBreakpointXL},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetHeightBreakpoint(tt.height)
			if got != tt.expected {
				t.Errorf("GetHeightBreakpoint(%d) = %d, want %d", tt.height, got, tt.expected)
			}
		})
	}
}

func TestResponsive_Get(t *testing.T) {
	r := Responsive[int]{
		XS:  1,
		SM:  2,
		MD:  3,
		LG:  4,
		XL:  5,
		XXL: 6,
	}

	tests := []struct {
		bp       Breakpoint
		expected int
	}{
		{BreakpointXS, 1},
		{BreakpointSM, 2},
		{BreakpointMD, 3},
		{BreakpointLG, 4},
		{BreakpointXL, 5},
		{BreakpointXXL, 6},
	}

	for _, tt := range tests {
		got := r.Get(tt.bp)
		if got != tt.expected {
			t.Errorf("Responsive.Get(%d) = %d, want %d", tt.bp, got, tt.expected)
		}
	}
}

func TestResponsive_Get_Default(t *testing.T) {
	r := Responsive[string]{
		XS:  "xs",
		SM:  "sm",
		MD:  "md",
		LG:  "lg",
		XL:  "xl",
		XXL: "xxl",
	}

	// Invalid breakpoint should return MD
	got := r.Get(Breakpoint(99))
	if got != "md" {
		t.Errorf("expected 'md' for invalid breakpoint, got %q", got)
	}
}

func TestFlexItem(t *testing.T) {
	item := FlexItem{
		Content: "test",
		Grow:    1,
		Shrink:  2,
		Basis:   100,
	}

	if item.Content != "test" {
		t.Errorf("expected 'test', got %q", item.Content)
	}
	if item.Grow != 1 {
		t.Errorf("expected grow 1, got %d", item.Grow)
	}
	if item.Shrink != 2 {
		t.Errorf("expected shrink 2, got %d", item.Shrink)
	}
	if item.Basis != 100 {
		t.Errorf("expected basis 100, got %d", item.Basis)
	}
}
