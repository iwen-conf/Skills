### Implementing Jetpack Compose Glimmer Components

#### Cards

Cards are a fundamental surface-based container in Glimmer used to group related
content, such as text, images, icons and actions into a single focal point. They
establish a clear depth plane (Z-axis) in the Glasses environment, providing a
stable background for text, images, and icons. Never embed a card within a List
Item.

##### Core Implementation Logic

- **Surface Hierarchy:** Cards are designed to sit on a base surface or within a list. They provide an automatic visual feedback when focused if an `onClick` lambda is provided.
- **Interactivity:**
  - **IF** the entire Card serves as a single trigger (e.g., a media item or a setting): **THEN** provide an `onClick` lambda to enable focus effects.
  - **ELSE:** Leave `onClick` as `null` if the Card contains multiple internal interactive elements (like separate buttons or switches) to avoid focus contention.

##### Technical Documentation Links

If you are creating a Glimmer Card component, read the:

- **Developer Guidance:** [Jetpack Compose Glimmer: Cards](android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/cards.md)
- **API Source Code:** Use [references/card-source.md](card-source.md).
- **Implementation Samples:** Use [references/card-samples-source.md](card-samples-source.md). (Basic and Interactive Card usage)

#### Buttons

Buttons are the primary triggers for discrete actions in Glimmer. They are
specifically optimized for the display glasses focus model, where a focus
highlight is added when focus is moved to the button using the touchpad or other
methods.

##### Core Implementation Logic

- There are two types of buttons to choose from in Glimmer:
  - **Standard buttons** (Required text label with optional leading or trailing icons). Use this when there is more space in the UI, or when the meaning of the button isn't clear without text.
  - **Icon buttons** (icon only). Only use icon buttons when the icon is clearly understandable.
- Both icon and standard buttons have default and toggle variants:
  - Default (use for single actions like "buy" or "navigate")
  - Toggle (use for things with selected states like mute buttons)

##### Technical Documentation Links

If you are creating a Glimmer Button component, read the:

- **Developer Guidance:** [Jetpack Compose Glimmer: Buttons](android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/buttons.md)
- **API Source Code:** Use [references/button-source.md](button-source.md).
- **Implementation Samples:** Use [references/button-samples-source.md](button-samples-source.md).

#### Title Chips

Chips are a pill-shaped, specialized labeling component designed to sit above a
`Card` or a group of content to provide a title. Use title chips to display
concise information like a short title, a name, or a status.

##### Guidelines and usage

- **Spatial Spacing:** When using a standalone `TitleChip` above content, you must use `TitleChipDefaults.AssociatedContentSpacing` (8.dp) to maintain the visual hierarchy.
- **Interactivity:** Title chips are purely for informational purposes, they cannot be targeted or activated for navigation.
- **Layout** Always center text in a title chip. Never let the title chip go to two lines, and truncate extra words. Keep the label to three words or less.

##### Technical Documentation Links

If you are creating a Glimmer Title Chip component, read the:

- **Developer Guidance:** [Jetpack Compose Glimmer: Title Chips](android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/title-chips.md)
- **API Source Code:** Use [references/titlechip-source.md](titlechip-source.md).
- **Implementation Samples:** Use [references/titlechip-samples-source.md](titlechip-samples-source.md). (Title Chips above group content)

#### Icons

Icons are visual symbols used to represent concepts, actions, or status in a
concise way. In Glimmer, icons and icon buttons are optimized for the XR
environment, providing clear visibility on additive displays and gaze-responsive
feedback for interactive elements.

##### Guidelines and usage

- **Sizing:** Use predefined `IconSizes` (e.g., Standard, Large) to ensure icons remain legible and meet the minimum touch target requirements for the XR environment.
- **Interactivity:**
  - **If** an icon serves as a trigger: **Then** you must use an `IconButton` to provide the automatic visual feedback and required tap-target padding.
  - **ELSE:** Use a standalone `Icon` for non-interactive indicators or status symbols.
- **Contrast:** **NEVER** use pure black (#000000) for icon tints; always use themed colors or standard Glimmer content colors to ensure the icon is visible against dark or real-world backgrounds.
- **Icon Library** The default icon library is [Material Symbols](https://fonts.google.com/icons?icon.style=Rounded) with 600 weight, Rounded, Unfilled, unless otherwise specified.

##### Technical Documentation Links

If you are creating a Glimmer Icon component, read the:

- **Developer Guidance:** [Jetpack Compose Glimmer: Icons](android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/icons.md)
- **API Source Code (Icon):** Use [references/icon-source.md](icon-source.md).
- **API Source Code (IconButton):** Use [references/iconbutton-source.md](iconbutton-source.md).
- **API Source Code (IconSizes):** Use [references/iconsizes-source.md](iconsizes-source.md).

#### Lists

Lists are containers that allow you to navigate between and see multiple items
on glasses. If your use case works with only seeing one item in the list at a
time, use a Glimmer Stack. Use lists with a Title Chip when the list
items are similar in type. Also use a Glimmer Stack if the items are of
different types.

##### Guidelines and usage

- **ListItem Slots:** Use the `ListItem` composable for rows. It provides predefined slots. Use [references/listitem-source.md](listitem-source.md) for reference.
- **Visual Consistency:** When building lists of similar items, always use a consistent background color (typically `GlimmerTheme.colors.surface`) and corner radius (standard 36.dp) for every item. Don't vary these unless you are visually grouping different *types* of content.
- **Integrated Title Chips:** Glimmer Lists support integrated title chips. **IF** you need a section header within a list: **THEN** enable the integrated title chip rather than adding a standalone `TitleChip` to maintain spatial consistency.
- **Vertical Arrangement:** ALWAYS use `verticalArrangement =
  Arrangement.spacedBy(20.dp)` for `VerticalList` to ensure visual separation between items on the glasses display.
- Be sure to use the default 20 dp spacing between list items unless otherwise specified.

##### Technical Documentation Links

If you are creating a Glimmer List component, read the:

- **API Source Code (List):** Use [references/list-source.md](list-source.md).
- **API Source Code (ListItem):** Use [references/listitem-source.md](listitem-source.md).
- **API Source Code (ListState):** Use [references/liststate-source.md](liststate-source.md).

#### Stacks

A stack is a collapsed list that only displays one piece of content at a time,
in a stacked visual, such as a card. If it is useful to show more than one item
at a time, use the Glimmer List control. Don't use a title chip with a stack.
If the items are of different types use a stack to show them. If the items are
of the same type, use a list.

##### Guidelines and usage

**Layout tips**

- Stacks accept variable height items.
- Align your stack control with the bottom of the display.
- The stack control must be 66 dp taller than the tallest item in the stack, allowing room for the scrim and minimizing movement when navigating between items.
- **Clipping and Layering:** To ensure that items behind the top item are correctly clipped and hidden, you **must** apply the `.itemDecoration(CardDefaults.shape)` modifier to the `Card` (or relevant container) inside every `item` block.
- **Depth Shadows:** To maintain spatial separation and element priority, utilize Glimmer's `ShadowScope`, `DepthEffect`, or prescribed `DepthEffectLevels` for depth plane scaling.

##### Technical Documentation Links

If you are creating a Glimmer Stack component, read the:

- **API Source Code (Stack):** Use [references/stack-source.md](stack-source.md).
- **API Source Code (StackState):** Use [references/stackstate-source.md](stackstate-source.md).
- **API Source Code (StackItemScope):** Use [references/stackitemscope-source.md](stackitemscope-source.md).

#### Text

In Jetpack Compose Glimmer, the [`Text`](android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/text.md) component builds off the Compose
text component, and lets you set various text properties. Be sure to choose a
style from the `GlimmerTheme` for your text. Modify the theme for your
application if you want custom typography.

##### Essential Constraint: Glimmer Text versus Material Text

On transparent Display Glasses (additive displays), standard Material `Text`
resolves to dark foreground tokens which render as transparent and invisible.
Glimmer `Text` intelligently manages theme color matching. When no manual color
override is specified, Glimmer `Text` automatically defaults to the content
color provided by the nearest Glimmer surface.

- **Don't:** Use Material Text
- **Do:** Use Glimmer Text

#### Surface

`Surface` is a fundamental building block in Glimmer. Use
[references/surface-source.md](surface-source.md) for reference.

A surface represents a distinct visual area or 'physical' boundary for
components such as buttons and cards. Use it if you need to build a custom
component.

