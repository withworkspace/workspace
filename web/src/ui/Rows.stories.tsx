import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "react-router";
import { Chip } from "./Chip";
import { Icon } from "./Icon";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "./Rows";
import { Status } from "./Status";
import { Text, textClass } from "./Text";

// A list of rows: each a link with its status, its title and what's beside
// it, a quieter line under it, and facts at its end; one picked, one still
// (with no link), one whose title runs long.

const meta: Meta = { title: "Components/Rows" };
export default meta;

export const List: StoryObj = {
  render: () => (
    <div className="page">
      <Rows role="listbox" aria-label="Missions">
        <Row role="option" aria-selected={false}>
          <Link to="/" className={rowLinkClass()}>
            <Status s={{ icon: "circle-dot", tone: "attention", words: "Needs you" }} iconOnly size={16} className={rowStatusClass()} />
            <RowMain>
              <RowLine>
                <RowTitle>Move the web UI to CSS modules</RowTitle>
                <Chip tone="quiet">
                  <Icon name="pr" size={11} />
                  #94
                </Chip>
              </RowLine>
              <RowSub>Buttons, chips and heads are the library's: rows are next.</RowSub>
            </RowMain>
            <Chip tone="attention">Needs you</Chip>
            <RowMeta>
              <Icon name="box" size={13} />
              workspace/css-modules
            </RowMeta>
            <Text dim className="age">2h</Text>
          </Link>
        </Row>
        <Row picked role="option" aria-selected>
          <Link to="/" className={rowLinkClass()}>
            <Status s={{ icon: "circle-half", tone: "progress", words: "In progress" }} iconOnly size={16} className={rowStatusClass()} />
            <RowMain>
              <RowLine>
                <RowTitle>Picked: the one chosen</RowTitle>
              </RowLine>
              <RowSub>Shown whether or not keys are moving.</RowSub>
            </RowMain>
            <Text dim className="age">5m</Text>
          </Link>
        </Row>
        <Row still>
          <Icon name="box" size={16} className={rowStatusClass(textClass("dim"))} />
          <RowMain>
            <RowLine>
              <RowTitle>A still row, with no link</RowTitle>
            </RowLine>
          </RowMain>
        </Row>
        <Row>
          <Link to="/" className={rowLinkClass()}>
            <Status s={{ icon: "check", tone: "done", words: "Done" }} iconOnly size={16} className={rowStatusClass()} />
            <RowMain>
              <RowLine>
                <RowTitle>A title long enough that it runs out of room before the end of the row and is cut short with an ellipsis</RowTitle>
              </RowLine>
            </RowMain>
            <Text dim className="age">3d</Text>
          </Link>
        </Row>
      </Rows>
    </div>
  ),
};
