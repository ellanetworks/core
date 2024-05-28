"use client";
import React, { FC, ReactNode } from "react";
import { Col, Row, } from "@canonical/react-components";

interface Props {
  title: string;
  children: ReactNode;
}

const PageHeader: FC<Props> = ({ title, children }) => {

  return (
      <Row>
        <Col size={8}>
          <div className="p-panel__header">
            <h1 className="p-panel__title">{title}</h1>
            <div className="p-panel__controls">
              {children}
            </div>
          </div>
        </Col>
      </Row>
  );
};
export default PageHeader;
