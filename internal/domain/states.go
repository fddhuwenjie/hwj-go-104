package domain

// 状态转换表：所有状态变更必须先经过 CanXxx 校验，
// 与 docs/02-状态转换表.md 保持一致。

var lotTransitions = map[LotStatus][]LotStatus{
	LotRegistered: {LotQueued, LotScrapped},
	LotQueued:     {LotRunning, LotOnHold, LotScrapped},
	LotRunning:    {LotWaiting, LotOnHold, LotScrapped},
	LotWaiting:    {LotQueued, LotOnHold, LotCompleted, LotScrapped},
	LotOnHold:     {LotQueued, LotWaiting, LotScrapped}, // 复判放行/返工后恢复
	LotCompleted:  {LotClosed},
	LotClosed:     {},
	LotScrapped:   {},
}

// CanLotTransition 判断批次状态转换是否合法。
func CanLotTransition(from, to LotStatus) bool {
	for _, s := range lotTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

var revisionTransitions = map[RevisionStatus][]RevisionStatus{
	RevDraft:   {RevActive, RevRetired},
	RevActive:  {RevRetired},
	RevRetired: {},
}

// CanRevisionTransition 判断路线修订状态转换是否合法。
func CanRevisionTransition(from, to RevisionStatus) bool {
	for _, s := range revisionTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

var recipeTransitions = map[RecipeStatus][]RecipeStatus{
	RecipeDraft:   {RecipeActive, RecipeRetired},
	RecipeActive:  {RecipeRetired},
	RecipeRetired: {},
}

// CanRecipeTransition 判断配方版本状态转换是否合法。
func CanRecipeTransition(from, to RecipeStatus) bool {
	for _, s := range recipeTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

var planTransitions = map[PlanStatus][]PlanStatus{
	PlanDraft:   {PlanActive, PlanRetired},
	PlanActive:  {PlanRetired},
	PlanRetired: {},
}

// CanPlanTransition 判断量测计划状态转换是否合法。
func CanPlanTransition(from, to PlanStatus) bool {
	for _, s := range planTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

var runTransitions = map[RunStatus][]RunStatus{
	RunRunning:   {RunCompleted, RunAborted},
	RunCompleted: {RunJudged, RunAborted},
	RunJudged:    {},
	RunAborted:   {},
}

// CanRunTransition 判断运行状态转换是否合法。
func CanRunTransition(from, to RunStatus) bool {
	for _, s := range runTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

var holdTransitions = map[HoldStatus][]HoldStatus{
	HoldOpen:     {HoldReleased, HoldReworked, HoldScrapped},
	HoldReleased: {},
	HoldReworked: {},
	HoldScrapped: {},
}

// CanHoldTransition 判断暂扣状态转换是否合法（复判只允许一次）。
func CanHoldTransition(from, to HoldStatus) bool {
	for _, s := range holdTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}
