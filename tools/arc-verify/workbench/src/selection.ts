export type CanvasMode = "dependencies" | "states" | "run";
export type FocusMode = "all" | "upstream" | "downstream" | "critical" | "blocked";
export type DependencyScope = "branch" | "flow" | "model";
export interface ObservedSelection {
  run: string;
  step: string;
  actor: string;
  action_index: number;
  branch: string;
  occurrence: string;
  entity_keys: string[];
}
export interface Selection {
  model: string;
  environment: string;
  flow: string;
  campaign: string;
  variant: string;
  branch: string;
  run: string;
  step: string;
  node: string;
  actor: string;
  entity: string;
  occurrence?: string;
  canvas: CanvasMode;
  focus: FocusMode;
  dependencyScope?: DependencyScope;
}
export interface SelectionStep {
  id: string;
  node: string;
  actors: string[];
  entities: string[];
  branches?: string[];
  actorNodes?: Record<string, string[]>;
  actorBranches?: Record<string, string[]>;
  preparation?: boolean;
}
export interface SelectionRun {
  id: string;
  branch: string;
  branches?: string[];
  flow: string;
  environment: string;
  round?: string;
  runner?: string;
  steps: SelectionStep[];
}
export interface SelectionCampaign {
  id: string;
  environment: string;
  flow: string;
  round: string;
  variants: { id: string; branch: string; runner: string }[];
  visits: { sequence: number; variant_id: string; branch: string; runner: string; run: string }[];
}
export interface SelectionBranch {
  id: string;
  action: string;
  flow: string;
}
export interface SelectionContext {
  model: string;
  environments: string[];
  branches: SelectionBranch[];
  runs: SelectionRun[];
  campaigns?: SelectionCampaign[];
}
export type SelectionAction =
  | {
      type:
        | "model"
        | "environment"
        | "flow"
        | "campaign"
        | "variant"
        | "visit"
        | "branch"
        | "run"
        | "step"
        | "node"
        | "lineage"
        | "occurrence"
        | "actor"
        | "entity";
      value: string;
    }
  | { type: "canvas"; value: CanvasMode }
  | { type: "focus"; value: FocusMode }
  | { type: "dependencyScope"; value: DependencyScope }
  | { type: "observed"; value: ObservedSelection };

export function initialSelection(model: string, environment = "", node = ""): Selection {
  return {
    model,
    environment,
    flow: "",
    campaign: "",
    variant: "",
    branch: "",
    run: "",
    step: "",
    node,
    actor: "",
    entity: "",
    occurrence: "",
    canvas: "states",
    focus: "all",
  };
}
export const matchesStep = (step: SelectionStep, state: Selection) =>
  (!state.actor || step.actors.includes(state.actor)) &&
  (!state.entity || step.entities.includes(state.entity));
export function matchingRuns(context: SelectionContext, state: Selection): SelectionRun[] {
  if (!state.branch) return [];
  const campaign = context.campaigns?.find((item) => item.id === state.campaign);
  if (state.campaign && !campaign) return [];
  return context.runs.filter((run) => {
    const visit = campaign?.visits.find(
      (item) => item.run === run.id && (!state.variant || item.variant_id === state.variant),
    );
    if (
      campaign &&
      (!visit ||
        run.environment !== campaign.environment ||
        run.round !== campaign.round ||
        (run.runner && run.runner !== visit.runner))
    )
      return false;
    return (
      (run.branch === state.branch ||
        run.branches?.includes(state.branch) ||
        visit?.branch === state.branch) &&
      (!state.environment || run.environment === state.environment) &&
      (!state.flow ||
        run.flow === state.flow ||
        run.steps.some((step) =>
          step.branches?.some(
            (branch) => context.branches.find((item) => item.id === branch)?.flow === state.flow,
          ),
        ))
    );
  });
}
function clearedRun(state: Selection): Selection {
  return { ...state, run: "", step: "", actor: "", entity: "", occurrence: "" };
}
function atStep(state: Selection, step?: SelectionStep, context?: SelectionContext): Selection {
  const branches = state.actor
    ? (step?.actorBranches?.[state.actor] ?? step?.branches)
    : step?.branches;
  const nodes = state.actor ? step?.actorNodes?.[state.actor] : undefined;
  // A partially frozen step stays diagnostic until a selected actor has a
  // complete recorded association; a known member cannot fill an unknown one.
  const branchID = branches?.includes("unresolved:records")
    ? "unresolved:records"
    : branches?.length === 1
      ? branches[0]
      : undefined;
  const branch = context?.branches.find((item) => item.id === branchID);
  return {
    ...state,
    occurrence: step?.id === state.step ? state.occurrence : "",
    step: step?.id ?? "",
    node: nodes ? (nodes.length === 1 ? nodes[0] : "") : (step?.node ?? state.node),
    branch: branch?.id ?? state.branch,
    flow: branch?.flow || state.flow,
    dependencyScope: step ? "branch" : state.dependencyScope,
  };
}

/** Every transition clears invalid dependent context. No run is guessed from a shared node. */
export function reduceSelection(
  state: Selection,
  action: SelectionAction,
  context: SelectionContext,
): Selection {
  if (action.type === "dependencyScope")
    return {
      ...state,
      dependencyScope: action.value,
      canvas: "dependencies",
      node: "",
      focus: "all",
    };
  if (action.type === "observed") {
    const value = action.value;
    const run = matchingRuns(context, state).find((item) => item.id === value.run);
    const step = run?.steps.find((item) => item.id === value.step);
    const branch = context.branches.find((item) => item.id === value.branch);
    const branches = step?.actorBranches?.[value.actor] ?? step?.branches ?? [];
    const key = JSON.stringify([
      value.run,
      Number(value.step),
      value.actor,
      value.action_index,
      "before",
      value.branch,
      null,
    ]);
    if (
      value.run !== state.run ||
      !step ||
      !branch ||
      !step.actors.includes(value.actor) ||
      !branches.includes(value.branch) ||
      !Number.isSafeInteger(value.action_index) ||
      value.action_index < 0 ||
      value.occurrence !== key ||
      (state.entity && !value.entity_keys.includes(state.entity))
    )
      return state;
    return {
      ...state,
      step: step.id,
      actor: value.actor,
      branch: branch.id,
      flow: branch.flow || state.flow,
      node: branch.action,
      occurrence: value.occurrence,
      canvas: "states",
      focus: "all",
      dependencyScope: "branch",
    };
  }
  if (action.type === "lineage") return { ...state, node: action.value };
  if (action.type === "occurrence") return { ...state, occurrence: action.value };
  if (action.type === "model")
    return action.value === context.model
      ? initialSelection(action.value, state.environment)
      : state;
  if (action.type === "environment") {
    if (action.value && !context.environments.includes(action.value)) return state;
    return { ...clearedRun(state), campaign: "", variant: "", environment: action.value };
  }
  if (action.type === "flow")
    return {
      ...clearedRun(state),
      campaign: "",
      variant: "",
      flow: action.value,
      branch: "",
      node: "",
      focus: "all",
      dependencyScope: "flow",
    };
  if (action.type === "campaign") {
    if (!action.value)
      return { ...clearedRun(state), campaign: "", variant: "", branch: "", node: "" };
    const campaign = context.campaigns?.find((item) => item.id === action.value);
    if (!campaign) return state;
    return {
      ...clearedRun(state),
      campaign: campaign.id,
      environment: campaign.environment,
      flow: campaign.flow,
      variant: "",
      branch: "",
      node: "",
      focus: "all",
    };
  }
  if (action.type === "variant") {
    if (!action.value) return { ...clearedRun(state), variant: "" };
    const campaign = context.campaigns?.find((item) => item.id === state.campaign);
    const variant = campaign?.variants.find((item) => item.id === action.value);
    const branch = context.branches.find((item) => item.id === variant?.branch);
    if (!campaign || !variant || !branch) return state;
    return {
      ...clearedRun(state),
      variant: variant.id,
      branch: branch.id,
      flow: campaign.flow,
      node: branch.action,
      focus: "all",
      dependencyScope: "branch",
    };
  }
  if (action.type === "visit") {
    const campaign = context.campaigns?.find((item) => item.id === state.campaign);
    const visit = campaign?.visits.find((item) => item.run === action.value);
    if (!visit) return state;
    const selectedVariant = reduceSelection(
      state,
      { type: "variant", value: visit.variant_id },
      context,
    );
    return reduceSelection(selectedVariant, { type: "run", value: visit.run }, context);
  }
  if (action.type === "branch") {
    if (!action.value)
      return { ...clearedRun(state), branch: "", variant: "", node: "", dependencyScope: "flow" };
    const branch = context.branches.find((item) => item.id === action.value);
    if (!branch) return state;
    return {
      ...clearedRun(state),
      branch: branch.id,
      variant: "",
      flow: branch.flow || state.flow,
      node: branch.action,
      focus: "all",
      dependencyScope: "branch",
    };
  }
  if (action.type === "run") {
    if (!action.value) return clearedRun(state);
    const run = matchingRuns(context, state).find((item) => item.id === action.value);
    if (!run) return state;
    const visit = context.campaigns
      ?.find((item) => item.id === state.campaign)
      ?.visits.find((item) => item.run === run.id);
    return atStep(
      {
        ...state,
        run: run.id,
        variant: visit?.variant_id ?? "",
        actor: "",
        entity: "",
        canvas: "run",
        focus: "all",
      },
      run.steps.find((step) => step.branches?.includes(state.branch)) ?? run.steps[0],
      context,
    );
  }
  if (action.type === "canvas")
    return {
      ...state,
      canvas: action.value,
      focus:
        (state.focus === "critical" || state.focus === "blocked") && action.value !== "dependencies"
          ? "all"
          : state.focus,
    };
  if (action.type === "focus")
    return (action.value === "critical" || action.value === "blocked") &&
      state.canvas !== "dependencies"
      ? state
      : { ...state, focus: action.value };
  const run = matchingRuns(context, state).find((item) => item.id === state.run);
  if (action.type === "node") {
    const step = run?.steps.find(
      (item) =>
        (state.actor && item.actorNodes?.[state.actor]
          ? item.actorNodes[state.actor].includes(action.value)
          : item.node === action.value ||
            Object.values(item.actorNodes ?? {}).some((nodes) => nodes.includes(action.value))) &&
        matchesStep(item, state),
    );
    return atStep({ ...state, node: action.value }, step, context);
  }
  if (action.type === "step") {
    const step = run?.steps.find((item) => item.id === action.value && matchesStep(item, state));
    return step ? atStep(state, step, context) : state;
  }
  if (action.type === "actor" || action.type === "entity") {
    if (!run) return state;
    const list = action.type === "actor" ? "actors" : "entities";
    if (action.value && !run.steps.some((step) => step[list].includes(action.value))) return state;
    const next = { ...state, [action.type]: action.value, occurrence: "" };
    const step =
      run.steps.find((item) => item.id === state.step && matchesStep(item, next)) ??
      run.steps.find((item) => matchesStep(item, next));
    return atStep(next, step, context);
  }
  return state;
}
