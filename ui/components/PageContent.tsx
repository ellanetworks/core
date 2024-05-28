"use client";
import React, { FC, ReactNode } from "react";
import { Col, Row, } from "@canonical/react-components";

interface Props {
  children: ReactNode;
}

const PageContent: FC<Props> = ({ children }) => {
  return (
    <Row>
      <Col size={6}>
        <div className="p-panel__content">
            {children}
        </div>
      </Col>
    </Row>
  );
};
export default PageContent;
