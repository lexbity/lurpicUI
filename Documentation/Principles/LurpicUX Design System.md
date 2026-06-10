# LurpicUX Design System

## Overview

LurpicUX is the design language and mark authoring model for LurpicUI.

LurpicUX Marks describes UX intent.
LurpicUI executes that intent through the facet projection runtime.

A LurpicUX mark is not merely a visual component. It is an authored projection contract: a reusable declaration of visual anatomy, layout behavior, interaction behavior, state ownership, theme slots, projection output, and environment adaptation.

The goal of LurpicUX is to support both conventional structured interfaces and freeform interactive systems such as graph canvases, chart editors, viewport overlays, annotations, inspectors, node editors, and professional tool interfaces.

## Relationship to LurpicUI

LurpicUX describes design intent.
LurpicUI executes that intent.

The mapping is:

- Design principle → mark authoring rule
- Mark → authored UX intent
- Facet → runtime projection boundary
- Role → participation in layout, projection, rendering, input, focus, or ticking
- Store → source of truth for shared state
- Theme recipe → maps mark intent/state/variant into resolved visual slots
- Projection → produces draw commands, hit regions, anchors, selection geometry, and child contexts

A LurpicUX element is therefore not only a visual component. It is a projection contract.

A projection contract describes how authored UX intent becomes:

- layout behavior
- visual output
- hit regions
- focus behavior
- input routing
- anchors
- selection geometry
- layer behavior
- theme slots
- state synchronization
- environment adaptation

## Authoring Concepts

Essential ideas and conflicting tensions when designing UX

### Adaptability vs Usage environment

UX is constructed to operate in different environments and constraints. 

This is often done by dynamically changing layout and what UX elements are present. This is triggered by an environment context.

Layout changes can involve how UX is re-organized:
- Resize
- Hide/show
- orientation

LurpicUX marks adapt to the environment they are projected into.

Environment context may include:

- viewport size
- input modality: mouse, touch, pen, keyboard, gamepad
- density: compact, comfortable, spacious
- color scheme: light, dark, high contrast
- locale and writing direction
- platform conventions
- motion preference
- accessibility settings
- projection context: pane, canvas, overlay, modal, chart, viewport

References:

https://m3.material.io/foundations/adaptive-design


### Readability vs. Compactness (information density)

Readable UX increases comprehension and accessibility.
Compact UX increases information density and professional efficiency.

LurpicUX does not choose one globally. Instead, marks must respond to density, environment, input mode, and user preference.

Design rule:
- Every stable mark should define at least compact and comfortable density behavior.
- Text-bearing marks must define truncation, wrapping, and minimum readable size.
- Dense modes may reduce spacing but must not remove essential affordances without an alternate path.

Runtime implications:
- Density is part of style context.
- Density may affect layout measurement.
- Density changes invalidate projection.

references:
https://m3.material.io/foundations/writing/best-practices
https://www.w3.org/WAI/WCAG22/quickref/?versions=2.1
https://m3.material.io/foundations/layout/understanding-layout/density


### Spacing vs Association (grouping)

References:
https://practicalpie.com/gestalt-principles/


### Affordance design: minimalism (visual consistancy) vs. realistic metaphor (skeumorph)

Many design systems seek to sanitize graphical aethetic for a universal graphical language. However, this also removes the 
meaning and relationship with the environment that is was derived from or interacted with.

A choice in itself: over sanitization removes the original context that the UX was designed for or to be; which limits affordances.  


examples of UX systems with a 'realistic metaphor' include:
- video games (https://www.gameuidatabase.com/gameData.php?id=1098  , https://medium.com/@gamer.express/fallout-3s-visual-design-makes-it-one-of-the-best-in-the-series-sklaper-540bbddff47e)
- DAW software with VST plugins 

### Localization and cultural design vs. Consistancy 

Not all cultures will interpet and recieve UX designs the same way.

References:
- https://spectrum.adobe.com/page/international-design/
- https://spectrum.adobe.com/page/bi-directionality/

### Interaction vs. all the above

Intended focus , triggers , readability , affordances affect interpetation and action. A balancing act of intent defining the properties of usability.

LurpicUX treats interaction behavior as part of design, not as an implementation detail.

A mark’s design contract includes:

- visual anatomy
- layout behavior
- hit regions
- focus behavior
- keyboard behavior
- pointer behavior
- gesture behavior
- state transitions
- motion feedback
- accessibility affordances
- theme slots
- projection/layer behavior

References:
https://m3.material.io/foundations/usability/overview

### Interaction Systems

A LurpicUX design is not always a page, panel, or component tree.

Some UX is structured:
- forms
- dialogs
- navigation
- tables
- inspectors

Some UX is freeform:
- graph canvases
- node editors
- chart annotations
- selection handles
- viewport overlays
- drawing tools
- timeline editors
- spatial inspectors

LurpicUX marks must work in both cases.

The same authored mark may project differently depending on whether it appears inside a pane, canvas, overlay, modal, chart, or viewport.


## Core concepts 

### Layout: Panes

Reference: 
https://m3.material.io/foundations/layout/applying-layout/pane-layouts
https://m3.material.io/foundations/layout/understanding-layout/parts-of-layout

### Layout: Grid 

Reference:
https://carbondesignsystem.com/elements/2x-grid/overview/

### Theme Model

Themes are not just color palettes. A LurpicUX theme resolves the visual language of marks.

Stable reusable marks must not directly choose colors, typography, spacing, or motion values. They consume resolved slots produced by theme recipes.

Direct styling is allowed only for raw graphics primitives, generated visualization marks, debug overlays, tests, and explicit escape hatches.

The canonical theme pipeline is:

Tokens → Materials → Recipes → Resolved Slots → Projection

Tokens
Raw design values such as color, typography, spacing, radius, elevation, motion, and density.

Materials
Semantic surface and foreground roles such as app background, panel, control, raised overlay, selection, warning, danger, and disabled.

Recipes
Mark-specific mappings from mark type, variant, state, density, and environment into resolved slots.

Resolved Slots
Concrete visual values consumed by projection: brushes, strokes, text styles, icon styles, radii, padding, elevation, and motion parameters.

 Projection
The mark/facet uses resolved slots to produce draw commands, hit regions, selection geometry, anchors, and child projection contexts.


Rules:

- Tokens are not a mark styling API.
- Materials are semantic design roles.
- Recipes map mark intent, variant, state, density, and environment into slots.
- Slots are the only stable styling surface consumed by reusable marks.
- Projection converts resolved slots into gfx commands.

Direct styling is allowed only for:
- raw graphics primitives
- generated visualization marks
- debug overlays
- tests
- explicit escape hatches

### color paletes 

Reference:
https://m3.material.io/styles/color/system/how-the-system-works
https://carbondesignsystem.com/data-visualization/color-palettes/

### Spacing 

Spacing affects readility of UX elements and typography

Reference:
https://carbondesignsystem.com/elements/spacing/overview/
https://m3.material.io/foundations/layout/understanding-layout/spacing

### Motion (animation)

Feedback effect upon interaction 

Reference:
https://spectrum.adobe.com/page/motion/
https://carbondesignsystem.com/elements/motion/overview/

### Iconagraphy

small identity symbols

Reference:
https://m3.material.io/styles/icons/overview
https://m3.material.io/styles/icons/applying-icons
https://spectrum.adobe.com/page/iconography/


### typography

What is a design system without one?

reference:
https://m3.material.io/styles/typography/editorial-treatments
https://spectrum.adobe.com/page/typography/


### Interaction input 

Different interactions offer different affordances to design

Reference:
https://m3.material.io/foundations/interaction/inputs

### Interaction state

Components show a feedback on what featureset is active

References:
https://spectrum.adobe.com/page/states/
https://m3.material.io/foundations/interaction/states/applying-states


## Mark Taxonomy and Authored Intent Families

Marks are grouped by authored intent, not by implementation.

'UX' Marks
Marks used for conventional interfaces: actions, selection, input, navigation, status, feedback, and structure.

Spatial (chart/canvas) Visualization marks
Marks used for data visualization, charts, graph canvases, annotations, axes, scales, legends, and interactive data affordances.

Some marks exist in both domains. For example, labels, badges, tooltips, handles, overlays, and selections may appear in structured UI or visualization contexts.

### primitives

Low-level visual or textual marks. They may be used directly or composed into higher-level marks.

- Text (label)
- image 
- rect 
- line
- path 
- polygon 
- ellipse/circle
- divider https://m3.material.io/components/divider/overview

### ux structure 

Marks that organize space, projection context, clipping, layering, anchoring, or grouping.


- cards https://spectrum.adobe.com/page/cards/ , https://m3.material.io/components/cards/guidelines
- panels/dockers https://docs.krita.org/en/reference_manual/dockers.html , 
- table https://spectrum.adobe.com/page/table/, https://carbondesignsystem.com/components/data-table/usage/
- list https://m3.material.io/components/lists/specs


### ux input 

- text input field - https://m3.material.io/components/text-fields/specs
- Link - https://carbondesignsystem.com/components/link/usage/

### ux action 

Marks that invoke commands or expose command surfaces.


- action bar - https://spectrum.adobe.com/page/action-bar/
- action group/FAB menu - https://spectrum.adobe.com/page/action-group/ , https://m3.material.io/components/fab-menu/specs

- toolbar/ribbon - https://en.wikipedia.org/wiki/Toolbar , https://en.wikipedia.org/wiki/Ribbon_(user_interface) , https://carbondesignsystem.com/patterns/text-toolbar-pattern/

- context menu / pop-up palette / sheets (mobile)  - https://en.wikipedia.org/wiki/Context_menu , https://docs.krita.org/en/reference_manual/popup-palette.html , https://m3.material.io/components/bottom-sheets/overview , https://m3.material.io/components/side-sheets/specs

- split button https://m3.material.io/components/split-button/overview , https://code.visualstudio.com/api/ux-guidelines/panel#panel-toolbar
- filter chips : https://m3.material.io/components/chips/guidelines
- search : https://m3.material.io/components/search/overview
- menu/menu button : https://carbondesignsystem.com/components/menu-buttons/usage/ , https://carbondesignsystem.com/components/menu/usage/
- command palette : https://code.visualstudio.com/api/ux-guidelines/command-palette


Reference:
https://carbondesignsystem.com/patterns/common-actions/

### ux navigation 
- breadcrumbs
- pagination
- tabs https://m3.material.io/components/tabs/specs, https://carbondesignsystem.com/components/content-switcher/usage/ , https://carbondesignsystem.com/components/tabs/usage/ 
- navigation drawer
- navigation rail
- tree navigator


### ux selection 

Marks that allow users to choose one or more values, objects, modes, or options.


- buttons https://m3.material.io/components/buttons/overview
- checkbox https://m3.material.io/components/checkbox/specs
- icon buttons https://m3.material.io/components/icon-buttons/overview
- list items  https://m3.material.io/components/lists/specs
- radio group (buttons) https://m3.material.io/components/radio-button/specs
- button group https://m3.material.io/components/button-groups/overview
- sliders https://m3.material.io/components/sliders/specs , https://carbondesignsystem.com/components/slider/usage/
- switches https://m3.material.io/components/switch/specs
- dropdown https://carbondesignsystem.com/components/dropdown/usage/

reference:
https://m3.material.io/foundations/interaction/selection
https://carbondesignsystem.com/patterns/filtering/

### ux status

Marks that communicate state without necessarily requiring action.


- badge/status light https://spectrum.adobe.com/page/badge/ , https://carbondesignsystem.com/components/tag/usage/ , https://carbondesignsystem.com/patterns/status-indicator-pattern/
- meter https://spectrum.adobe.com/page/meter/ ,  https://m3.material.io/components/progress-indicators/overview
- progress bar https://spectrum.adobe.com/page/progress-bar/ , https://m3.material.io/components/loading-indicator/overview

### ux feedback 

Marks that respond to events, errors, confirmations, progress, or blocking decisions.


- notification https://carbondesignsystem.com/components/notification/usage/ , https://carbondesignsystem.com/patterns/notification-pattern/
- dialog https://m3.material.io/components/dialogs/specs , https://carbondesignsystem.com/components/modal/usage/ , https://carbondesignsystem.com/patterns/dialog-pattern/
- tooltip https://m3.material.io/components/tooltips/specs , https://carbondesignsystem.com/components/tooltip/usage/


### spatial shape 

- polygons https://d3js.org/d3-polygon
- arc 
- line 
- pie 
- stack

Reference:
https://d3js.org/d3-shape
https://carbondesignsystem.com/data-visualization/chart-types/
https://carbondesignsystem.com/data-visualization/chart-anatomy/

### spatial axis

Reference:
https://d3js.org/d3-axis
https://d3js.org/d3-scale
https://carbondesignsystem.com/data-visualization/axes-and-labels/


## Mark Contract

Every stable mark must define:

1. Authored intent
2. Family and construction class
3. Semantic anatomy
4. Visual anatomy
5. Composition strategy
6. Theme slots and recipe contract
7. Layout contract
8. Typography contract, if text is rendered
9. Projection contract
10. Hit and focus contract
11. Input and gesture contract
12. Anchor contract, if anchors are exported or consumed
13. Layer contract, if overlays, clipping, elevation, or portals are involved
14. State ownership contract
15. Environment adaptation behavior
16. Localization and directionality behavior, if text/order/direction matters
17. Accessibility behavior
18. Motion behavior, if animated
19. Diagnostics contract
20. Golden screenshot and replay coverage

### Principles

1. Intent before implementation  
   Marks describe what UX role is needed before they describe how it is drawn.

2. Projection before rendering  
   A mark projects interaction geometry, hit regions, anchors, and child contexts, not only pixels.

3. Slots before style values  
   Stable marks consume resolved theme slots, not raw colors or ad hoc typography.

4. Stores own truth  
   Shared state lives in stores. Marks and facets own translation and transient interaction state.

5. Environment changes projection  
   Density, input mode, accessibility settings, locale, projection context, and viewport constraints may change how a mark resolves.

6. Interaction is design  
   Focus, hit regions, keyboard behavior, gestures, and state transitions are part of the design contract.

7. Reuse requires evidence  
   A stable mark needs golden screenshots, replay scenarios, and diagnostic overlays.

8. Semantic anatomy before visual anatomy
A mark first defines the parts of its UX meaning, then defines how those parts are visually projected.

9. Composition must be explicit
A mark must declare whether it resolves into one facet, multiple child facets, generated children, or portal/layer-mounted children.

10. Typography is layout
Text measurement, shaping, baseline alignment, wrapping, truncation, caret geometry, and selection geometry are layout/projection concerns, not decorative styling details.

11. Layers are interaction boundaries
Layers affect not only render order, but hit testing, focus behavior, clipping, modality, dismissal, and coordinate space.
   
### State Ownership

Marks may own local interaction state.
Marks must not own shared domain truth.

Shared observable state lives in stores.
Facet-local state may include hover, press, focus visual state, drag draft state, and transient animation state.

A mark must declare:
- store reads
- store writes or emitted events
- local facet state
- derived state
- forbidden state
