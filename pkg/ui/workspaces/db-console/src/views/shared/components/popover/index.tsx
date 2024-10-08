// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

import classNames from "classnames";
import React from "react";

import { OutsideEventHandler } from "src/components/outsideEventHandler";

import "./popover.styl";

export interface PopoverProps {
  content: JSX.Element;
  visible: boolean;
  onVisibleChange: (nextState: boolean) => void;
  children: any;
}

export default class Popover extends React.Component<PopoverProps> {
  // contentRef is used to pass as element to avoid handling outside event handler
  // on its instance.
  contentRef: React.RefObject<HTMLDivElement> = React.createRef();

  render() {
    const { content, children, visible, onVisibleChange } = this.props;

    const popoverClasses = classNames("popover", {
      "popover--visible": visible,
    });

    return (
      <React.Fragment>
        <div
          ref={this.contentRef}
          className="popover__content"
          onClick={() => onVisibleChange(!visible)}
        >
          {content}
        </div>
        <OutsideEventHandler
          onOutsideClick={() => onVisibleChange(false)}
          mountNodePosition={"fixed"}
          ignoreClickOnRefs={[this.contentRef]}
        >
          <div className={popoverClasses}>{children}</div>
        </OutsideEventHandler>
      </React.Fragment>
    );
  }
}
