package httpcallback

// func HTTPDeregistrationNotification(c *gin.Context) {
// 	var deregistrationData models.DeregistrationData

// 	requestBody, err := c.GetRawData()
// 	if err != nil {
// 		logger.CallbackLog.Errorf("Get Request Body error: %+v", err)
// 		problemDetail := models.ProblemDetails{
// 			Title:  "System failure",
// 			Status: http.StatusInternalServerError,
// 			Detail: err.Error(),
// 			Cause:  "SYSTEM_FAILURE",
// 		}
// 		c.JSON(http.StatusInternalServerError, problemDetail)
// 		return
// 	}

// 	err = openapi.Deserialize(&deregistrationData, requestBody, "application/json")
// 	if err != nil {
// 		problemDetail := "[Request Body] " + err.Error()
// 		rsp := models.ProblemDetails{
// 			Title:  "Malformed request syntax",
// 			Status: http.StatusBadRequest,
// 			Detail: problemDetail,
// 		}
// 		logger.CallbackLog.Errorln(problemDetail)
// 		c.JSON(http.StatusBadRequest, rsp)
// 		return
// 	}

// 	req := httpwrapper.NewRequest(c.Request, deregistrationData)
// 	if supi, exists := c.Params.Get("supi"); exists {
// 		req.Params["supi"] = supi
// 	}
// 	rsp := producer.HandleDeregistrationNotification(req)

// 	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
// 	if err != nil {
// 		logger.CallbackLog.Errorln(err)
// 		problemDetails := models.ProblemDetails{
// 			Status: http.StatusInternalServerError,
// 			Cause:  "SYSTEM_FAILURE",
// 			Detail: err.Error(),
// 		}
// 		c.JSON(http.StatusInternalServerError, problemDetails)
// 	} else {
// 		c.Data(rsp.Status, "application/json", responseBody)
// 	}
// }
