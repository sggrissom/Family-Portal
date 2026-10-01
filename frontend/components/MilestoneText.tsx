import * as preact from "preact";
import * as server from "../server";
import { block } from "vlens/css";

block(`
.milestone-quote {
  font-style: italic;
}
`);

export const isQuote = (milestone: server.Milestone) => milestone.category === "quote";

export const MilestoneText = ({ milestone }: { milestone: server.Milestone }) =>
  isQuote(milestone) ? (
    <q className="milestone-quote">{milestone.description}</q>
  ) : (
    <>{milestone.description}</>
  );
