export type CanvasMode = "dependencies" | "states" | "run";
export type FocusMode = "all" | "upstream" | "downstream" | "critical";
export interface Selection {
  model: string;
  environment: string;
  flow: string;
  branch: string;
  run: string;
  step: string;
  node: string;
  actor: string;
  entity: string;
  canvas: CanvasMode;
  focus: FocusMode;
}
export interface SelectionStep {
  id: string;
  node: string;
  actors: string[];
  entities: string[];
  branches?: string[];
  actorNodes?: Record<string, string[]>;
  actorBranches?: Record<string, string[]>;
}
export interface SelectionRun {
  id: string;
  branch: string;
  branches?: string[];
  flow: string;
  environment: string;
  steps: SelectionStep[];
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
}
export type SelectionAction =
  | {
      type:
        | "model"
        | "environment"
        | "flow"
        | "branch"
        | "run"
        | "step"
        | "node"
        | "actor"
        | "entity";
      value: string;
    }
  | { type: "canvas"; value: CanvasMode }
  | { type: "focus"; value: FocusMode };

export function initialSelection(model: string, environment = "", node = ""): Selection {
  return {
    model,
    environment,
    flow: "",
    branch: "",
    run: "",
    step: "",
    node,
    actor: "",
    entity: "",
    canvas: "states",
    focus: "all",
  };
}
export const matchesStep = (step: SelectionStep, state: Selection) =>
  (!state.actor || step.actors.includes(state.actor)) &&
  (!state.entity || step.entities.includes(state.entity));
export function matchingRuns(context: SelectionContext, state: Selection): SelectionRun[] {
  if (!state.branch) return [];
  return context.runs.filter(
    (run) =>
      (run.branch === state.branch || run.branches?.includes(state.branch)) &&
      (!state.environment || run.environment === state.environment) &&
      (!state.flow ||
        run.flow === state.flow ||
        run.steps.some((step) =>
          step.branches?.some(
            (branch) => context.branches.find((item) => item.id === branch)?.flow === state.flow,
          ),
        )),
  );
}
function clearedRun(state: Selection): Selection {
  return { ...state, run: "", step: "", actor: "", entity: "" };
}
function atStep(state: Selection, step?: SelectionStep, context?: SelectionContext): Selection {
  const branches = state.actor
    ? (step?.actorBranches?.[state.actor] ?? step?.branches)
    : step?.branches;
  const nodes = state.actor ? step?.actorNodes?.[state.actor] : undefined;
  const branch =
    branches?.length === 1 ? context?.branches.find((item) => item.id === branches[0]) : undefined;
  return {
    ...state,
    step: step?.id ?? "",
    node: nodes ? (nodes.length === 1 ? nodes[0] : "") : (step?.node ?? state.node),
    branch: branch?.id ?? state.branch,
    flow: branch?.flow || state.flow,
  };
}

/** Every transition clears invalid dependent context. No run is guessed from a shared node. */
export function reduceSelection(
  state: Selection,
  action: SelectionAction,
  context: SelectionContext,
): Selection {
  if (action.type === "model")
    return action.value === context.model
      ? initialSelection(action.value, state.environment)
      : state;
  if (action.type === "environment") {
    if (action.value && !context.environments.includes(action.value)) return state;
    return { ...clearedRun(state), environment: action.value };
  }
  if (action.type === "flow")
    return { ...clearedRun(state), flow: action.value, branch: "", node: "", focus: "all" };
  if (action.type === "branch") {
    if (!action.value) return { ...clearedRun(state), branch: "", node: "" };
    const branch = context.branches.find((item) => item.id === action.value);
    if (!branch) return state;
    return {
      ...clearedRun(state),
      branch: branch.id,
      flow: branch.flow,
      node: branch.action,
      focus: "all",
    };
  }
  if (action.type === "run") {
    if (!action.value) return clearedRun(state);
    const run = matchingRuns(context, state).find((item) => item.id === action.value);
    if (!run) return state;
    return atStep(
      { ...state, run: run.id, actor: "", entity: "", canvas: "run", focus: "all" },
      run.steps.find((step) => step.branches?.includes(state.branch)) ?? run.steps[0],
      context,
    );
  }
  if (action.type === "canvas")
    return {
      ...state,
      canvas: action.value,
      focus: state.focus === "critical" && action.value !== "dependencies" ? "all" : state.focus,
    };
  if (action.type === "focus")
    return action.value === "critical" && state.canvas !== "dependencies"
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
    const next = { ...state, [action.type]: action.value };
    const step =
      run.steps.find((item) => item.id === state.step && matchesStep(item, next)) ??
      run.steps.find((item) => matchesStep(item, next));
    return atStep(next, step, context);
  }
  return state;
}
