import { test } from "vite-plus/test";
import assert from "node:assert/strict";

import {
  createPosterStateMap,
  createPosterMonogram,
  getPosterStateKey,
  markPosterLoaded,
  markPosterFailed,
  POSTER_FAILED,
  POSTER_LOADED,
  POSTER_LOADING,
  resolvePosterElementState,
  shouldShowPoster,
} from "./poster-state.ts";

test("createPosterStateMap seeds loading for shows with poster URLs and failed for shows without posters", () => {
  const shows = [
    { id: 7, poster: "https://img.example/poster.jpg" },
    { id: 9, poster: "" },
  ];

  const states = createPosterStateMap(shows);

  assert.equal(states[getPosterStateKey(shows[0])], "loading");
  assert.equal(states[getPosterStateKey(shows[1])], "failed");
});

test("createPosterStateMap preserves resolved states for unchanged poster keys", () => {
  const show = { id: 11, poster: "https://img.example/naruto.jpg" };
  const key = getPosterStateKey(show);

  const states = createPosterStateMap([show], {
    [key]: "loaded",
  });

  assert.equal(states[key], "loaded");
});

test("createPosterStateMap resets changed poster URLs back to loading", () => {
  const previous = {
    "13::https://img.example/old.jpg": "loaded",
  };
  const show = { id: 13, poster: "https://img.example/new.jpg" };

  const states = createPosterStateMap([show], previous);

  assert.equal(states[getPosterStateKey(show)], "loading");
});

test("markPosterLoaded stores a loaded state for a poster key", () => {
  const show = { id: 17, poster: "https://img.example/one-piece.jpg" };

  const states = markPosterLoaded({}, show);

  assert.equal(states[getPosterStateKey(show)], "loaded");
});

test("markPosterFailed stores a failed state for a poster key", () => {
  const show = { id: 21, poster: "https://img.example/pokemon.jpg" };

  const states = markPosterFailed({}, show);

  assert.equal(states[getPosterStateKey(show)], "failed");
});

test("shouldShowPoster only returns true for loaded posters", () => {
  assert.equal(shouldShowPoster("loading"), false);
  assert.equal(shouldShowPoster("failed"), false);
  assert.equal(shouldShowPoster("loaded"), true);
});

test("createPosterMonogram returns up to two initials for poster placeholders", () => {
  assert.equal(createPosterMonogram("Detective Conan"), "DC");
  assert.equal(createPosterMonogram(" One   Piece "), "OP");
  assert.equal(createPosterMonogram("Naruto"), "N");
});

test("resolvePosterElementState returns loading while an image is still pending", () => {
  assert.equal(
    resolvePosterElementState({ hasPoster: true, complete: false, naturalWidth: 0 }),
    POSTER_LOADING,
  );
});

test("resolvePosterElementState returns loaded for a completed image with natural width", () => {
  assert.equal(
    resolvePosterElementState({ hasPoster: true, complete: true, naturalWidth: 320 }),
    POSTER_LOADED,
  );
});

test("resolvePosterElementState returns failed for a completed broken image or missing poster", () => {
  assert.equal(
    resolvePosterElementState({ hasPoster: true, complete: true, naturalWidth: 0 }),
    POSTER_FAILED,
  );
  assert.equal(
    resolvePosterElementState({ hasPoster: false, complete: false, naturalWidth: 0 }),
    POSTER_FAILED,
  );
});
