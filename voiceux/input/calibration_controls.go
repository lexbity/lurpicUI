package input

import "codeburg.org/lexbit/voicedsp"

// CalibrationStep identifies one calibration prompt.
type CalibrationStep struct {
	Vowel voicedsp.Vowel
	Label string
	Index int
	Total int
}

var defaultCalibrationSteps = []CalibrationStep{
	{Vowel: voicedsp.VowelA, Label: "A / ah", Index: 0, Total: 5},
	{Vowel: voicedsp.VowelE, Label: "E / eh", Index: 1, Total: 5},
	{Vowel: voicedsp.VowelI, Label: "I / ee", Index: 2, Total: 5},
	{Vowel: voicedsp.VowelO, Label: "O / oh", Index: 3, Total: 5},
	{Vowel: voicedsp.VowelU, Label: "U / oo", Index: 4, Total: 5},
}

// DefaultCalibrationSteps returns the canonical A/E/I/O/U prompt flow.
func DefaultCalibrationSteps() []CalibrationStep {
	out := make([]CalibrationStep, len(defaultCalibrationSteps))
	copy(out, defaultCalibrationSteps)
	return out
}

// NextCalibrationStep returns the next prompt in the canonical flow.
func NextCalibrationStep(current voicedsp.Vowel) (CalibrationStep, bool) {
	for i, step := range defaultCalibrationSteps {
		if step.Vowel == current && i+1 < len(defaultCalibrationSteps) {
			return defaultCalibrationSteps[i+1], true
		}
	}
	return CalibrationStep{}, false
}
