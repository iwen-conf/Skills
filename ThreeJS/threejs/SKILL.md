---
name: threejs
description: >-
  Three.js and React Three Fiber work: scenes, cameras, renderers (WebGL and WebGPU), math, raycasting, controls,
  geometries, loaders (GLTF, Draco, KTX2), materials and shaders (GLSL, TSL), lighting, shadows, post-processing,
  animation, physics, audio, drei, WebXR, IFC/BIM viewers, scene building and model optimization, plus fixing
  rendering errors and performance problems. 3D 网页, 三维场景. Not for general frontend pages (use arc:code) or
  visual direction (use ui-design).
license: MIT
metadata:
  library:
    - r3f-best-practices
    - threejs-agents-model-optimizer
    - threejs-agents-scene-builder
    - threejs-core-math
    - threejs-core-raycaster
    - threejs-core-renderer
    - threejs-core-scene-graph
    - threejs-errors-performance
    - threejs-errors-rendering
    - threejs-impl-animation
    - threejs-impl-audio
    - threejs-impl-drei
    - threejs-impl-ifc-viewer
    - threejs-impl-lighting
    - threejs-impl-physics
    - threejs-impl-post-processing
    - threejs-impl-react-three-fiber
    - threejs-impl-shadows
    - threejs-impl-webgpu
    - threejs-impl-xr
    - threejs-syntax-controls
    - threejs-syntax-geometries
    - threejs-syntax-loaders
    - threejs-syntax-materials
    - threejs-syntax-shaders
---

# threejs

One entry for Three.js and React Three Fiber. Pick the row for the symptom or API in front of you and read only
that entry (or the two or three a row lists, in order). Entries are MIT-licensed guides from the OpenAEC
Foundation; skip their API tables when the code at hand already shows the call, and lean on their anti-patterns
and version notes. Verify version-specific APIs against the project's installed `three` version before writing
code; code changes follow `arc:code`.

## Route

| Request | Entry |
|---|---|
| Symptom: black screen, nothing renders, wrong colors, z-fighting, broken textures | [threejs-errors-rendering](../../Library/threejs/threejs-errors-rendering/SKILL.md) |
| Symptom: low FPS, memory growth, too many draw calls, disposal leaks | [threejs-errors-performance](../../Library/threejs/threejs-errors-performance/SKILL.md) |
| Scene graph, transforms, groups, object lifecycle | [threejs-core-scene-graph](../../Library/threejs/threejs-core-scene-graph/SKILL.md) |
| Vectors, matrices, quaternions, Euler angles, coordinate spaces | [threejs-core-math](../../Library/threejs/threejs-core-math/SKILL.md) |
| Renderer setup, color management, pixel ratio, render loop / WebGPU renderer | [threejs-core-renderer](../../Library/threejs/threejs-core-renderer/SKILL.md) / [threejs-impl-webgpu](../../Library/threejs/threejs-impl-webgpu/SKILL.md) |
| Picking and pointer interaction | [threejs-core-raycaster](../../Library/threejs/threejs-core-raycaster/SKILL.md) |
| Camera controls (Orbit, Trackball, Pointer Lock) | [threejs-syntax-controls](../../Library/threejs/threejs-syntax-controls/SKILL.md) |
| Geometries / loaders (GLTF, Draco, KTX2, textures) | [threejs-syntax-geometries](../../Library/threejs/threejs-syntax-geometries/SKILL.md) / [threejs-syntax-loaders](../../Library/threejs/threejs-syntax-loaders/SKILL.md) |
| Materials / custom shaders (GLSL, TSL, ShaderMaterial) | [threejs-syntax-materials](../../Library/threejs/threejs-syntax-materials/SKILL.md) / [threejs-syntax-shaders](../../Library/threejs/threejs-syntax-shaders/SKILL.md) |
| Lights / shadows / post-processing passes | [threejs-impl-lighting](../../Library/threejs/threejs-impl-lighting/SKILL.md) / [threejs-impl-shadows](../../Library/threejs/threejs-impl-shadows/SKILL.md) / [threejs-impl-post-processing](../../Library/threejs/threejs-impl-post-processing/SKILL.md) |
| Animation mixer and clips / physics engines / positional audio | [threejs-impl-animation](../../Library/threejs/threejs-impl-animation/SKILL.md) / [threejs-impl-physics](../../Library/threejs/threejs-impl-physics/SKILL.md) / [threejs-impl-audio](../../Library/threejs/threejs-impl-audio/SKILL.md) |
| React Three Fiber: Canvas, useFrame, declarative patterns / R3F project conventions / drei helpers | [threejs-impl-react-three-fiber](../../Library/threejs/threejs-impl-react-three-fiber/SKILL.md) / [r3f-best-practices](../../Library/threejs/r3f-best-practices/SKILL.md) / [threejs-impl-drei](../../Library/threejs/threejs-impl-drei/SKILL.md) |
| WebXR (VR/AR sessions, controllers) | [threejs-impl-xr](../../Library/threejs/threejs-impl-xr/SKILL.md) |
| IFC / BIM model viewing | [threejs-impl-ifc-viewer](../../Library/threejs/threejs-impl-ifc-viewer/SKILL.md) |
| Build a whole scene from a brief / optimize a 3D model for the web | [threejs-agents-scene-builder](../../Library/threejs/threejs-agents-scene-builder/SKILL.md) / [threejs-agents-model-optimizer](../../Library/threejs/threejs-agents-model-optimizer/SKILL.md) |

## Completion

- [ ] Entry chosen by symptom or API; `three` / `@react-three/fiber` versions stated when behavior depends on them.
- [ ] Rendering or performance fixes show before/after evidence (screenshot, FPS, draw calls, memory).
