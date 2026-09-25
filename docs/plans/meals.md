# Meal tracking with kid ratings

Track what gets cooked and what everyone actually thought of it, then use the
history to decide what to make.

Scope:
- Meals composed of parts — entree, sides, dessert — each rated on its own. A
  kid can like the chicken and hate the green beans, and averaging that into one
  score loses the useful information.
- Optional ingredients and method per component, so this doubles as a recipe
  box.
- A cooked-meal record: date plus which components were served.
- Ratings per person per component, 1-5, entered after the meal.
- Photos, reusing the existing photo pipeline.

The payoff is the queries, so build with those in mind:
- What does a specific picky kid actually like?
- What does everyone like that hasn't been made in a while?
- What's never been rated, so it's untested?
- What is nobody's favorite and can quietly retire?

Open question: whether ratings are per-cooking or per-component-overall. Per
cooking is more honest (the same dish varies) and can still be averaged up.
