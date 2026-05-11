# Proposal: About Page

**Status:** Draft
**Author:** agent
**Date:** 2026-05-11
**Scope:** Frontend only

---

## Summary

Add a static `/about` page to the Enshor frontend with hero section, five content sections, and a "How it works" card grid. Add an "About" nav item to the existing layout navbar.

---

## Scope

- New route `/about` in `App.tsx`
- New page component `src/pages/About.tsx`
- Add "About" nav link in `Layout.tsx` (desktop + mobile)
- Use existing Tailwind utilities, custom CSS classes (`card`, `gradient-text`, `gradient-line`, `animate-fade-in-up`), and Solana color tokens (`solana-purple`, `solana-green`, `solana-dark`, `solana-card`, `solana-border`)
- Use existing brand asset `public/brand/enshor-about-visual.png`

## Non-goals

- No backend changes
- No Solana program changes
- No new dependencies
- No auth, wallet, campaign, or claim logic changes
- No API calls on the About page
- No routing test updates unless existing tests explicitly cover nav item assertions

---

## Architecture

### Routing

Current pattern in `App.tsx` (react-router-dom v6):

```tsx
<Route path="/" element={<Home />} />
<Route path="/create" element={<CreateCampaign />} />
<Route path="/campaign/:id" element={<CampaignDetails />} />
<Route path="/profile" element={<Profile />} />
```

Add:

```tsx
<Route path="/about" element={<About />} />
```

Import `About` from `./pages/About`.

### Navigation

Current nav items in `Layout.tsx` are rendered via a `navLink(path, label)` helper at lines 43-47 (desktop) and 81-85 (mobile). Add `{navLink('/about', 'About')}` after the Profile link in both locations.

### Page structure

`About.tsx` — single functional component, no hooks, no state, no API calls.

Layout sections:

1. **Hero** — flex row on desktop (text left, image right), flex col on mobile. Title with `gradient-text` or `text-white`. Subtitle and intro paragraph.
2. **What Enshor does** — prose section with max-width constraint.
3. **Why the name Enshor** — prose section.
4. **How it works** — 4 cards in a 2x2 grid (`grid-cols-1 sm:grid-cols-2`). Each card uses `card` class from `index.css`.
5. **Why Solana** — prose section.
6. **MVP scope** — prose section.

### Styling tokens

Reuse existing Tailwind config and `index.css` classes:

| Token | Usage |
|---|---|
| `solana-dark` | Page background (inherited from Layout) |
| `solana-card` | Card backgrounds |
| `solana-border` | Card borders, section dividers |
| `solana-purple` | Headings, accents |
| `solana-green` | Gradient accents |
| `text-white` | Section titles |
| `text-gray-500` / `text-gray-400` | Body text |
| `gradient-text` | Hero title |
| `gradient-line` | Section dividers |
| `card` | How-it-works cards |
| `animate-fade-in-up` | Entry animations |
| `max-w-3xl` or `max-w-2xl` | Prose width constraint |

---

## API contracts

None. The About page is purely static — no API calls.

---

## Data model

None. No data fetching or state.

---

## Config / env strategy

No new env vars or config needed.

---

## Error handling

No error states. The page is static with no async operations.

---

## Validation

None. No form inputs or user data.

---

## Security notes

- The hero image path `/brand/enshor-about-visual.png` is a static asset served from `public/`. No user-supplied URLs.
- No dynamic content injection.

---

## Acceptance criteria

1. Route `/about` renders the About page.
2. Desktop nav bar shows: Campaigns | Create | Profile | About — matching existing nav link styles.
3. Mobile nav bar shows the same items.
4. Active nav link highlights when on `/about` (purple bg, existing `navLink` behavior).
5. Hero section: title "About Enshor" with gradient text, subtitle, intro text on the left; image on the right on desktop; stacked vertically on mobile.
6. Image uses `/brand/enshor-about-visual.png`, has `alt` text, and is responsive.
7. Five content sections with exact copy as specified.
8. "How it works" section has 4 cards in a responsive grid (1 col mobile, 2 cols desktop).
9. Each card has a title and body text, using the `card` CSS class.
10. Prose sections have a readable max-width (~`max-w-3xl`).
11. Page uses dark surfaces, subtle borders, gradients consistent with the rest of the app.
12. Page feels compact and polished, not like a long document.
13. `npm run build` passes with zero errors.
14. No new dependencies added.

---

## Implementation tasks

### Task 1: Create `src/pages/About.tsx`

- Create the page component with all six sections (hero + 5 content sections).
- Use exact copy from the spec.
- Hero: `flex flex-col md:flex-row gap-8 md:gap-12 items-center`. Text block on left with `flex-1`. Image block on right with `flex-1` and `max-w-md` or similar.
- Image: `<img src="/brand/enshor-about-visual.png" alt="Enshor visual" className="w-full rounded-xl" />`
- Section titles: `text-xl font-semibold text-white` with a `gradient-line` below each.
- Prose: `max-w-3xl text-gray-400 text-sm leading-relaxed`.
- Cards: `grid grid-cols-1 sm:grid-cols-2 gap-4`. Each card uses `card` class, title `text-sm font-semibold text-white`, body `text-xs text-gray-400`.
- Use `animate-fade-in-up` with staggered `animationDelay` on sections for polish.
- Wrap in a `div` with `space-y-16` or explicit spacing between sections.

### Task 2: Add route in `src/App.tsx`

- Import `About` from `./pages/About`.
- Add `<Route path="/about" element={<About />} />` before the catch-all `*` route.

### Task 3: Add nav link in `src/components/Layout.tsx`

- Add `{navLink('/about', 'About')}` after the Profile link in the desktop nav (line 46 area).
- Add `{navLink('/about', 'About')}` after the Profile link in the mobile nav (line 84 area).

### Task 4: Verify build

- Run `npm run build` and confirm zero errors.

---

## Risks

| Risk | Mitigation |
|---|---|
| Image asset not found at build | Asset already exists at `public/brand/enshor-about-visual.png` — verified. |
| Nav link order feels wrong | Place "About" after "Profile" — standard for secondary/info links. |
| Page too long / document-like | Use compact spacing, section dividers, and card grid to break up content. Enforce max-width on prose. |
| Existing tests break | Only two test files exist (`CreateCampaign.test.tsx`, `githubRepoInput.test.ts`, `client.test.ts`, `GitHubAutocomplete.test.tsx`). None test routing or nav. No test changes expected. |

---

## Verification steps

1. `npm run build` — must pass with zero errors.
2. Manual: navigate to `/about` — page renders with all sections.
3. Manual: check desktop nav shows "About" with correct active state.
4. Manual: check mobile nav shows "About".
5. Manual: resize browser — hero stacks vertically on mobile, side-by-side on desktop.
6. Manual: card grid is 1 col on mobile, 2 cols on desktop.
7. `npm run lint` — no new warnings.

---

## Runtime strategy

- **Local tools:** Node.js (already installed), npm (existing `package-lock.json`).
- **Build verification:** `npm run build` locally.
- **No Docker needed.** This is a frontend-only static page addition.
- **Fallback:** If local Node is missing, use Docker with `node:20-alpine`. Unlikely — project already builds locally.

---

## Generator strategy

- No generators needed. This is a single-page addition to an existing project.
- Follow existing patterns: functional component in `src/pages/`, Tailwind classes, `index.css` custom classes.
- No template scaffolding required.

---

## Deployment packaging

TBD. The existing `Dockerfile` and `nginx.conf` in `frontend/` handle production builds. No changes to deployment config are needed for this feature.
