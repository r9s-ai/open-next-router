package dslconfig

import "fmt"

type modeRegistryState struct {
	usage             usageModeRegistry
	usagePaths        map[string]string
	finishReason      finishReasonModeRegistry
	finishReasonPaths map[string]string
	models            modelsModeRegistry
	modelsPaths       map[string]string
	balance           balanceModeRegistry
	balancePaths      map[string]string
}

func newModeRegistryState() modeRegistryState {
	return modeRegistryState{
		usage:             usageModeRegistry{},
		usagePaths:        map[string]string{},
		finishReason:      finishReasonModeRegistry{},
		finishReasonPaths: map[string]string{},
		models:            modelsModeRegistry{},
		modelsPaths:       map[string]string{},
		balance:           balanceModeRegistry{},
		balancePaths:      map[string]string{},
	}
}

func loadGlobalModeRegistryState(path string) (modeRegistryState, string, error) {
	state := newModeRegistryState()
	usage, usagePaths, globalContent, err := loadGlobalUsageModesFromFile(path)
	if err != nil {
		return modeRegistryState{}, "", err
	}
	finishReason, finishReasonPaths, _, err := loadGlobalFinishReasonModesFromFile(path)
	if err != nil {
		return modeRegistryState{}, "", err
	}
	models, modelsPaths, _, err := loadGlobalModelsModesFromFile(path)
	if err != nil {
		return modeRegistryState{}, "", err
	}
	balance, balancePaths, _, err := loadGlobalBalanceModesFromFile(path)
	if err != nil {
		return modeRegistryState{}, "", err
	}
	if usage != nil {
		state.usage = usage
	}
	if usagePaths != nil {
		state.usagePaths = usagePaths
	}
	if finishReason != nil {
		state.finishReason = finishReason
	}
	if finishReasonPaths != nil {
		state.finishReasonPaths = finishReasonPaths
	}
	if models != nil {
		state.models = models
	}
	if modelsPaths != nil {
		state.modelsPaths = modelsPaths
	}
	if balance != nil {
		state.balance = balance
	}
	if balancePaths != nil {
		state.balancePaths = balancePaths
	}
	return state, globalContent, nil
}

func parseLocalModeRegistryState(path string, content string) (modeRegistryState, error) {
	state := newModeRegistryState()
	var err error
	state.usage, err = parseGlobalUsageModes(path, content)
	if err != nil {
		return modeRegistryState{}, err
	}
	state.finishReason, err = parseGlobalFinishReasonModes(path, content)
	if err != nil {
		return modeRegistryState{}, err
	}
	state.models, err = parseGlobalModelsModes(path, content)
	if err != nil {
		return modeRegistryState{}, err
	}
	state.balance, err = parseGlobalBalanceModes(path, content)
	if err != nil {
		return modeRegistryState{}, err
	}
	return state, nil
}

func (s modeRegistryState) clone() modeRegistryState {
	cloned := newModeRegistryState()
	for name, cfg := range s.usage {
		cloned.usage[name] = cfg
	}
	for name, path := range s.usagePaths {
		cloned.usagePaths[name] = path
	}
	for name, cfg := range s.finishReason {
		cloned.finishReason[name] = cfg
	}
	for name, path := range s.finishReasonPaths {
		cloned.finishReasonPaths[name] = path
	}
	for name, cfg := range s.models {
		cloned.models[name] = cfg
	}
	for name, path := range s.modelsPaths {
		cloned.modelsPaths[name] = path
	}
	for name, cfg := range s.balance {
		cloned.balance[name] = cfg
	}
	for name, path := range s.balancePaths {
		cloned.balancePaths[name] = path
	}
	return cloned
}

func (s *modeRegistryState) merge(path string, local modeRegistryState) error {
	for name, cfg := range local.usage {
		if prev, ok := s.usagePaths[name]; ok {
			return fmt.Errorf("duplicate usage_mode %q in %q (already in %q)", name, path, prev)
		}
		s.usage[name] = cfg
		s.usagePaths[name] = path
	}
	for name, cfg := range local.finishReason {
		if prev, ok := s.finishReasonPaths[name]; ok {
			return fmt.Errorf("duplicate finish_reason_mode %q in %q (already in %q)", name, path, prev)
		}
		s.finishReason[name] = cfg
		s.finishReasonPaths[name] = path
	}
	for name, cfg := range local.models {
		if prev, ok := s.modelsPaths[name]; ok {
			return fmt.Errorf("duplicate models_mode %q in %q (already in %q)", name, path, prev)
		}
		s.models[name] = cfg
		s.modelsPaths[name] = path
	}
	for name, cfg := range local.balance {
		if prev, ok := s.balancePaths[name]; ok {
			return fmt.Errorf("duplicate balance_mode %q in %q (already in %q)", name, path, prev)
		}
		s.balance[name] = cfg
		s.balancePaths[name] = path
	}
	return nil
}

func (s modeRegistryState) resolve() (modeRegistryState, error) {
	resolved := newModeRegistryState()
	usage, err := resolveUsageModeRegistry(s.usagePaths, s.usage)
	if err != nil {
		return modeRegistryState{}, err
	}
	finishReason, err := resolveFinishReasonModeRegistry(s.finishReasonPaths, s.finishReason)
	if err != nil {
		return modeRegistryState{}, err
	}
	models, err := resolveModelsModeRegistry(s.modelsPaths, s.models)
	if err != nil {
		return modeRegistryState{}, err
	}
	balance, err := resolveBalanceModeRegistry(s.balancePaths, s.balance)
	if err != nil {
		return modeRegistryState{}, err
	}
	resolved.usage = usage
	resolved.usagePaths = s.usagePaths
	resolved.finishReason = finishReason
	resolved.finishReasonPaths = s.finishReasonPaths
	resolved.models = models
	resolved.modelsPaths = s.modelsPaths
	resolved.balance = balance
	resolved.balancePaths = s.balancePaths
	return resolved, nil
}
