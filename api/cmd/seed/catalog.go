package main

import "meso/api/models"

// muscleCatalog is the muscle lookup: every muscle a movement can tag, grouped by
// region. region drives coarse filtering ("posterior chain") and a future body
// map. Sourced from the anatomy references in ~/shart/fitness. This is reference
// data with no CLI verb behind it, so — unlike metrics and movements, which are
// content loaded through the `meso` CLI — it lives in the seed alongside the enum
// lookups.
var muscleCatalog = []models.Muscle{
	{Name: "chest", Region: "anterior"},
	{Name: "front_delts", Region: "anterior"},
	{Name: "quads", Region: "anterior"},
	{Name: "hip_flexors", Region: "anterior"},
	{Name: "abs", Region: "core"},
	{Name: "obliques", Region: "core"},
	{Name: "lats", Region: "posterior"},
	{Name: "upper_back", Region: "posterior"},
	{Name: "lower_back", Region: "posterior"},
	{Name: "traps", Region: "posterior"},
	{Name: "rear_delts", Region: "posterior"},
	{Name: "glutes", Region: "posterior"},
	{Name: "hamstrings", Region: "posterior"},
	{Name: "calves", Region: "posterior"},
	{Name: "shoulders", Region: "shoulders"},
	{Name: "side_delts", Region: "shoulders"},
	{Name: "biceps", Region: "arms"},
	{Name: "triceps", Region: "arms"},
	{Name: "forearms", Region: "arms"},
	{Name: "adductors", Region: "legs"},
	{Name: "abductors", Region: "legs"},
	{Name: "neck", Region: "other"},
}
