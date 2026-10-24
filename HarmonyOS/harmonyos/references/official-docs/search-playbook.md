# Search playbook

Use this when a HarmonyOS question needs more than one obvious exact-symbol lookup.

## Query ladder

Move down this ladder only when the previous step is insufficient.

1. **Exact identifier** — class, method, decorator, permission, error code, JSON5 key.
   - `VpnExtensionAbility`
   - `onWindowStageCreate`
   - `ohos.permission.INTERNET`
2. **Identifier + intent** — pair the exact symbol with the behavior being investigated.
   - `VpnExtensionAbility lifecycle`
   - `startAbility permission`
3. **Two related identifiers as separate searches** — do not overstuff one query.
   - `UIAbility`
   - `Want`
4. **Canonical concept terms** — use HarmonyOS terminology rather than conversational wording.
   - `background task continuous task`
   - `Stage model extension ability`
5. **Chinese/English counterpart** — when the index appears language-sensitive, retry the canonical term in the other language.

## Evidence selection

Prefer hits that directly contain the requested symbol and the specific fact you need. For implementation-critical questions, collect enough evidence to cover the relevant dimensions:

- declaration/signature and parameter types;
- required permission or capability;
- lifecycle and threading/context constraints;
- API version / device scope / system capability when present;
- failure modes or error codes;
- configuration entries required in `module.json5`, `app.json5`, or other build files.

Do not assume one hit covers all of these.

## Query sizing

Default to `--limit 5`. Increase to 8-10 only when the topic is broad or terminology is uncertain. Avoid dumping dozens of documents into context: it reduces answer quality and increases the chance of mixing unrelated APIs.

## Broadening and narrowing

If results are weak:

- remove prose and keep the rarest platform terms;
- search an exact error code separately;
- search the containing type and the member independently;
- search permission/config key independently from the API;
- try the official Chinese/English term pair.

If results are noisy:

- add the exact type/member;
- split one broad query into multiple narrow searches;
- focus on a lifecycle stage, permission, capability, or config key.

## Conflict handling

When documents appear inconsistent:

1. Check whether they describe different API levels, device categories, model generations, or lifecycle contexts.
2. Prefer the document that directly names the symbol and target scenario.
3. Report remaining ambiguity explicitly. Do not merge incompatible instructions into a synthetic answer.
