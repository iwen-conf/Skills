/**
 * Interactability & contrast verification snippets for Chrome DevTools injection.
 *
 * Usage: inject this script into the page via mcp_chrome-devtools_evaluate_script, then call the
 * exported functions from the console. Results are JSON objects suitable for
 * logging to events.jsonl (kind: "interactability_check").
 */

/* eslint-disable no-unused-vars */

/**
 * Check whether a single element is truly interactable.
 * Returns a JSON-serialisable result object.
 *
 * @param {string} selector - CSS selector for the target element
 * @returns {object} check results with pass/fail per criterion
 */
function checkInteractability(selector) {
  var el = document.querySelector(selector);
  if (!el) {
    return {
      selector: selector,
      found: false,
      interactable: false,
      checks: {},
      message: "Element not found: " + selector,
    };
  }

  var rect = el.getBoundingClientRect();
  var style = window.getComputedStyle(el);

  var isDisplayed = style.display !== "none";
  var isVisible = style.visibility !== "hidden" && style.visibility !== "collapse";
  var hasSize = rect.width > 0 && rect.height > 0;
  var hasOpacity = parseFloat(style.opacity) > 0;
  var hasPointerEvents = style.pointerEvents !== "none";
  var isEnabled = !el.disabled && !el.hasAttribute("aria-disabled");

  // Viewport check
  var vw = window.innerWidth || document.documentElement.clientWidth;
  var vh = window.innerHeight || document.documentElement.clientHeight;
  var inViewport =
    rect.top < vh && rect.bottom > 0 && rect.left < vw && rect.right > 0;

  // Obstruction check via elementFromPoint
  var cx = rect.left + rect.width / 2;
  var cy = rect.top + rect.height / 2;
  var topEl = document.elementFromPoint(cx, cy);
  var notObscured = topEl === el || el.contains(topEl);

  // Minimum touch target size (WCAG 2.5.8 Target Size)
  var MIN_TARGET = 24;
  var meetsMinTargetSize = rect.width >= MIN_TARGET && rect.height >= MIN_TARGET;

  var checks = {
    is_displayed: isDisplayed,
    is_visible: isVisible,
    has_size: hasSize,
    has_opacity: hasOpacity,
    has_pointer_events: hasPointerEvents,
    is_enabled: isEnabled,
    in_viewport: inViewport,
    not_obscured: notObscured,
    meets_min_target_size: meetsMinTargetSize,
  };

  var allPassed = Object.keys(checks).every(function (k) {
    // meets_min_target_size is advisory, not blocking
    return k === "meets_min_target_size" || checks[k];
  });

  return {
    selector: selector,
    found: true,
    interactable: allPassed,
    checks: checks,
    rect: {
      x: Math.round(rect.x),
      y: Math.round(rect.y),
      width: Math.round(rect.width),
      height: Math.round(rect.height),
    },
    message: allPassed
      ? "Element is interactable"
      : "Element failed checks: " +
        Object.keys(checks)
          .filter(function (k) {
            return !checks[k];
          })
          .join(", "),
  };
}

/**
 * Batch-check multiple selectors.
 *
 * @param {string[]} selectors - Array of CSS selectors
 * @returns {object} aggregate results
 */
function checkAllInteractable(selectors) {
  var results = selectors.map(checkInteractability);
  var allPassed = results.every(function (r) {
    return r.interactable;
  });
  return {
    total: selectors.length,
    passed: results.filter(function (r) {
      return r.interactable;
    }).length,
    failed: results.filter(function (r) {
      return !r.interactable;
    }).length,
    all_passed: allPassed,
    results: results,
  };
}

/**
 * Compute WCAG 2.x contrast ratio between an element's foreground and background.
 *
 * @param {string} selector - CSS selector for the target element
 * @returns {object} contrast check results
 */
function checkContrast(selector) {
  var el = document.querySelector(selector);
  if (!el) {
    return {
      selector: selector,
      found: false,
      contrast_ratio: null,
      message: "Element not found: " + selector,
    };
  }

  var style = window.getComputedStyle(el);
  var fg = _parseColor(style.color);
  var bg = _getEffectiveBackground(el);

  if (!fg || !bg) {
    return {
      selector: selector,
      found: true,
      foreground: style.color,
      background: bg ? _colorToString(bg) : "unknown",
      contrast_ratio: null,
      message: "Could not determine effective colors",
    };
  }

  var ratio = _contrastRatio(fg, bg);
  var fontSize = parseFloat(style.fontSize);
  var fontWeight = parseInt(style.fontWeight, 10) || 400;
  var isLargeText =
    fontSize >= 24 || (fontSize >= 18.66 && fontWeight >= 700);

  return {
    selector: selector,
    found: true,
    foreground: _colorToString(fg),
    background: _colorToString(bg),
    contrast_ratio: Math.round(ratio * 100) / 100,
    font_size_px: fontSize,
    is_large_text: isLargeText,
    wcag_aa: isLargeText ? ratio >= 3.0 : ratio >= 4.5,
    wcag_aaa: isLargeText ? ratio >= 4.5 : ratio >= 7.0,
    message:
      "Contrast " +
      ratio.toFixed(2) +
      ":1 — AA " +
      (isLargeText ? ratio >= 3.0 : ratio >= 4.5 ? "PASS" : "FAIL") +
      ", AAA " +
      (isLargeText ? ratio >= 4.5 : ratio >= 7.0 ? "PASS" : "FAIL"),
  };
}

// ── Internal helpers ──

function _parseColor(cssColor) {
  var m = cssColor.match(
    /rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*[\d.]+)?\s*\)/
  );
  if (m) return { r: parseInt(m[1]), g: parseInt(m[2]), b: parseInt(m[3]) };
  return null;
}

function _getEffectiveBackground(el) {
  var current = el;
  while (current && current !== document.documentElement) {
    var bg = window.getComputedStyle(current).backgroundColor;
    var parsed = _parseColor(bg);
    if (parsed && (parsed.r !== 0 || parsed.g !== 0 || parsed.b !== 0)) {
      // Check alpha
      var alphaMatch = bg.match(
        /rgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*([\d.]+)\s*\)/
      );
      if (!alphaMatch || parseFloat(alphaMatch[1]) > 0) return parsed;
    }
    current = current.parentElement;
  }
  return { r: 255, g: 255, b: 255 }; // default white
}

function _srgbLinear(c8bit) {
  var s = c8bit / 255;
  return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
}

function _relativeLuminance(color) {
  return (
    0.2126 * _srgbLinear(color.r) +
    0.7152 * _srgbLinear(color.g) +
    0.0722 * _srgbLinear(color.b)
  );
}

function _contrastRatio(fg, bg) {
  var l1 = _relativeLuminance(fg);
  var l2 = _relativeLuminance(bg);
  var lighter = Math.max(l1, l2);
  var darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

function _colorToString(c) {
  return "rgb(" + c.r + ", " + c.g + ", " + c.b + ")";
}
