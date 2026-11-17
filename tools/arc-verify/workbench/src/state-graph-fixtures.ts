import paymentModel from "../../examples/pay-gateway/model.json";
import typedBranchModel from "../dev/typed-branch-model.json";
import { contractFixture } from "./test-fixtures";
import type { Snapshot } from "./types";

function withModel(model: Snapshot["model"]["data"]): Snapshot {
  const snapshot = contractFixture();
  snapshot.model.data = structuredClone(model);
  snapshot.meta.graph = model.id;
  snapshot.runs = [];
  snapshot.business_campaigns = [];
  snapshot.contract_analysis = undefined;
  snapshot.contract_execution = undefined;
  snapshot.catalog.runs = [];
  snapshot.views.verification.obligations = [];
  return snapshot;
}

/** Uses the shipped source-backed example; this is UI projection, not product execution. */
export const paymentStateSnapshot = () =>
  withModel(paymentModel as unknown as Snapshot["model"]["data"]);

/** Export of the Go contractModel fixture, accepted by validateGraph; no real execution claim. */
export const typedBranchStateSnapshot = () =>
  withModel(typedBranchModel as unknown as Snapshot["model"]["data"]);
