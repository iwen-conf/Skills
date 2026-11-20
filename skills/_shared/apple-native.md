# Apple Native Hard-Spec Cheatsheet (iOS / iPadOS / macOS / visionOS)

Build with native `Swift + SwiftUI` (bridging `UIKit` / `AppKit` only where system capabilities require it). Never wrap web shells.

## 1. Clarity, Deference, Depth & Liquid Glass

- Follow `ui-system.md`. Keep controls secondary to content; reserve **Liquid Glass** and translucent system materials strictly for floating toolbars, tab bars, sheets, and chrome over scrolling content—never content backgrounds or nested cards.

## 2. Platform Ergonomics

- **iOS / iPadOS**: `>= 44pt x 44pt` hit targets, thumb-zone primary actions, safe-area & keyboard avoidance, `NavigationSplitView` adaptation across compact/regular size classes and dual-screen/foldable postures.
- **macOS**: Multi-window `Scene` commands, native menu bar shortcuts, pointer-grade dense lists/inspectors, full keyboard focus rings.
- **visionOS**: Place interactive controls within comfortable field of view; design for eyes-and-hands gaze focus + pinch with generous spacing so adjacent targets never collide.

## 3. Typography, SF Symbols & Concurrency

- Use `SF Pro` / `New York` semantic text styles (`>= 11pt`, ban Ultralight/Thin) with automatic horizontal-to-vertical stack reflow at accessibility Dynamic Type sizes.
- Pair text with weight/scale-matched `SF Symbols`; never convey status by color alone; set explicit `accessibilityLabel` for VoiceOver on icon-only controls.
- Use `@Observable`, structured Swift concurrency (`async/await`, `.task` lifetime cancellation), and `Sendable` actor isolation.
