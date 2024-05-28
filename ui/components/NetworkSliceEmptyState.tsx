"use client";
import React, { useState } from "react";
import NetworkSliceModal from "@/components/NetworkSliceModal";
import { Button, Row, Col, Strip } from "@canonical/react-components";

const NetworkSliceEmptyState = () => {
  const [isModalVisible, setIsModalVisible] = useState(false);

  const toggleModal = () => {
    setIsModalVisible(!isModalVisible);
  };

  return (
    <div>
      <table>
        <caption>
          <Strip>
            <Row>
              <Col size={8} medium={4} small={3} className="u-align--left">
                <p className="p-heading--4">No network slices available</p>
                <Button
                  appearance="positive"
                  className="mt-8"
                  onClick={toggleModal}
                >
                  Create
                </Button>
              </Col>
            </Row>
          </Strip>
        </caption>
      </table>

      {isModalVisible && (
        <NetworkSliceModal
          toggleModal={toggleModal}
        />
      )}
    </div>
  );
};
export default NetworkSliceEmptyState;
