package process

import (
	"github.com/AsterNighT/software-engineering-backend/api"
	"github.com/AsterNighT/software-engineering-backend/pkg/account"
	_ "github.com/AsterNighT/software-engineering-backend/pkg/cases"
	"github.com/AsterNighT/software-engineering-backend/pkg/utils"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"math"
	"net/http"
	"strconv"
)

type ProcessHandler struct{}


// GetAllDepartments
// @Summary get all departments
// @Tags Process
// @Description display all departments of a hospital
// @Produce json
// @Success 200 {object} api.ReturnedData{data=[]Department}
// @Router /departments [GET]
func (h *ProcessHandler) GetAllDepartments(c echo.Context) error {
	db := utils.GetDB()

	// get all departments
	var departments []Department
	db.Find(&departments)

	return c.JSON(http.StatusOK, api.Return("ok", departments))
}

// GetDepartmentByID
// @Summary get a department by its ID
// @Tags Process
// @Description return a department's details by its ID
// @Param departmentID path uint true "department ID"
// @Produce json
// @Success 200 {object} api.ReturnedData{data=DepartmentDetailJSON}
// @Router /department/{DepartmentID} [GET]
func (h *ProcessHandler) GetDepartmentByID(c echo.Context) error {
	db := utils.GetDB()
	var department Department                  // department basic information
	var schedules []DepartmentSchedule         // schedules
	var doctorsAll = make([]account.Doctor, 0) // doctor total information
	var doctors = make([]string, 0)            // doctor's name (show to frontend)

	err := db.First(&department, c.Param("departmentID")).Error
	if err != nil {
		return c.JSON(http.StatusNotFound, api.Return("error", DepartmentNotFound))
	}
	db.Where("department_id = ?", department.ID).Find(&schedules)
	db.Where("department_id = ?", department.ID).Find(&doctorsAll)

	for _, doctor := range doctorsAll {
		var a account.Account
		err = db.First(&a, doctor.AccountID).Error
		if err == nil {
			doctors = append(doctors, a.LastName+a.FirstName)
		}
	}

	// wrap the return data
	returnDepartment := DepartmentDetailJSON{
		ID:        department.ID,
		Name:      department.Name,
		Detail:    department.Detail,
		Doctors:   doctors,
		Schedules: schedules,
	}

	c.Logger().Debug("GetDepartmentByID")
	return c.JSON(http.StatusOK, api.Return("ok", returnDepartment))
}

// CreateRegistrationTX
// @Summary create registration
// @Tags Process
// @Description return registration state
// @Param department_id body uint true "department ID"
// @Param year body int true "Year"
// @Param month body int true "Month"
// @Param day body int true "Day"
// @Param halfday body int true "HalfDay"
// @Produce json
// @Success 200 {object} api.ReturnedData{data=RegistrationDetailJSON}
// @Router /registration [POST]
func (h *ProcessHandler) CreateRegistrationTX(c echo.Context) error {
	type RegistrationSubmitJSON struct {
		DepartmentID uint        `json:"department_id"`
		Year         int         `json:"year"`
		Month        int         `json:"month"`
		Day          int         `json:"day"`
		HalfDay      HalfDayEnum `json:"halfday" validate:"halfday"`
	}

	// extract submit data
	var submit RegistrationSubmitJSON
	if err := c.Bind(&submit); err != nil {
		c.Logger().Debug("JSON format failed when trying to create a registration ...")
		return c.JSON(http.StatusBadRequest, api.Return("error", InvalidSubmitFormat))
	}

	// validate halfday enum
	err := validate.Struct(submit)
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", InvalidSubmitFormat))
	}

	var res = 1

	db := utils.GetDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		// get department
		var department Department
		err = db.First(&department, submit.DepartmentID).Error
		if err != nil {
			c.Logger().Error(DepartmentNotFound)
			return err
		}

		// get patient
		var patient account.Patient
		err = db.Where("account_id = ?", c.Get("id").(uint)).First(&patient).Error
		if err != nil {
			c.Logger().Error(PatientNotFound)
			return err
		}

		var possibleDuplicates []Registration

		//check duplicate registration
		if db.Where(&Registration{
			DepartmentID: department.ID,
			PatientID:    patient.ID,
			Year:         submit.Year,
			Month:        submit.Month,
			Day:          submit.Day,
			HalfDay:      submit.HalfDay,
		}).Find(&possibleDuplicates).RowsAffected > 0 {
			for i := range possibleDuplicates {
				if possibleDuplicates[i].Status != terminated {
					//return c.JSON(http.StatusBadRequest, api.Return("error", DuplicateRegistration))
					return echo.ErrBadRequest
				}
			}
		}

		// check schedule
		var schedule DepartmentSchedule
		err = db.Where(&DepartmentSchedule{
			DepartmentID: department.ID,
			Year:         submit.Year,
			Month:        submit.Month,
			Day:          submit.Day,
			HalfDay:      submit.HalfDay,
		}).First(&schedule).Error

		// invalid schedule return and unlock
		if err != nil || !validateSchedule(&schedule) {
			c.Logger().Error(InvalidRegistration)
			return err
		} else if schedule.Current >= schedule.Capacity {
			c.Logger().Error(NotEnoughCapacity)
			return err
		}

		// assign the doctor with the minimal registrations
		var doctors []account.Doctor
		db.Where("department_id = ?", department.ID).Find(&doctors)
		var doctorRegistrationCount = make([]int64, len(doctors))

		registration := Registration{
			DepartmentID: department.ID,
			Year:         submit.Year,
			Month:        submit.Month,
			Day:          submit.Day,
			HalfDay:      submit.HalfDay,
		}

		// find min count of registrations
		minCount, minIndex := int64(math.MaxInt64), -1
		for i := range doctors {
			db.Model(&Registration{}).Where(&registration).Count(&doctorRegistrationCount[i])
			if minCount > doctorRegistrationCount[i] {
				minCount = doctorRegistrationCount[i]
				minIndex = i
			}
		}

		// cannot find a doctor
		if minIndex == -1 {
			c.Logger().Error(CannotAssignDoctor)
			return echo.ErrBadRequest
		}

		// process schedule
		registration.PatientID = patient.ID
		registration.DoctorID = doctors[minIndex].ID

		if err := db.Create(&registration).Error; err != nil {
			c.Logger().Error(InvalidRegistration)
			return err
		}

		schedule.Current = schedule.Current + 1
		if err := db.Save(&schedule).Error; err != nil {
			c.Logger().Error(InvalidSchedule)
			return err
		}

		// get name
		var patientAccount account.Account
		var doctor account.Doctor
		var doctorAccount account.Account
		db.First(&patientAccount, registration.PatientID)
		db.First(&doctor, registration.DoctorID)
		db.First(&doctorAccount, doctor.AccountID)

		res = int(registration.ID)

		return nil
	})

	if err != nil {
		c.Logger().Debug("JSON format failed when trying to create a registration ...")
		return c.JSON(http.StatusBadRequest, api.Return("error", CreateRegistrationFailed))
	} else {
		return c.JSON(http.StatusOK, api.Return("ok", res))
	}

}

// GetRegistrationsByPatient
// @Summary get all registrations (patient view)
// @Tags Process
// @Description display all registrations of a patient
// @Produce json
// @Success 200 {object} api.ReturnedData{data=[]RegistrationDetailJSON}
// @Router /patient/registrations [GET]
func (h *ProcessHandler) GetRegistrationsByPatient(c echo.Context) error {
	db := utils.GetDB()
	var registrations []Registration

	// get patient
	var patient account.Patient
	err := db.Where("account_id = ?", c.Get("id").(uint)).First(&patient).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", PatientNotFound))
	}

	db.Where("patient_id = ?", patient.ID).Find(&registrations)
	var registrationJSONs = make([]RegistrationJSON, len(registrations))

	for i := range registrations {
		var department Department
		db.First(&department, registrations[i].DepartmentID)
		registrationJSONs[i] = RegistrationJSON{
			ID:         registrations[i].ID,
			Department: department.Name,
			Status:     registrations[i].Status,
			Year:       registrations[i].Year,
			Month:      registrations[i].Month,
			Day:        registrations[i].Day,
			HalfDay:    registrations[i].HalfDay,
		}
	}
	c.Logger().Debug("GetRegistrationsByPatient")
	return c.JSON(http.StatusOK, api.Return("ok", registrationJSONs))
}

// GetRegistrationsByDoctor
// @Summary get all registrations (doctor view)
// @Tags Process
// @Description display all registrations of a patient
// @Produce json
// @Success 200 {object} api.ReturnedData{data=[]Registration}
// @Router /doctor/registrations [GET]
func (h *ProcessHandler) GetRegistrationsByDoctor(c echo.Context) error {
	db := utils.GetDB()
	var registrations []Registration

	// get patient
	var doctor account.Doctor
	err := db.Where("account_id = ?", c.Get("id").(uint)).First(&doctor).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", DoctorNotFound))
	}

	db.Where("doctor_id = ?", doctor.ID).Find(&registrations)
	var registrationJSONs = make([]RegistrationJSON, len(registrations))

	for i := range registrations {
		var department Department
		db.First(&department, registrations[i].DepartmentID)
		registrationJSONs[i] = RegistrationJSON{
			ID:         registrations[i].ID,
			Department: department.Name,
			Status:     registrations[i].Status,
			Year:       registrations[i].Year,
			Month:      registrations[i].Month,
			Day:        registrations[i].Day,
			HalfDay:    registrations[i].HalfDay,
		}
	}
	c.Logger().Debug("GetRegistrationsByDoctor")
	return c.JSON(http.StatusOK, api.Return("ok", registrationJSONs))
}

// NOTE: use cookie for identification

// GetRegistrationByPatient
// @Summary get a registration by its ID (patient view)
// @Tags Process
// @Description return a registration details by its ID
// @Param registrationID path uint true "registration's ID"
// @Produce json
// @Success 200 {object} api.ReturnedData{data=Registration}
// @Router /patient/registration/{registrationID} [GET]
func (h *ProcessHandler) GetRegistrationByPatient(c echo.Context) error {
	db := utils.GetDB()

	// get patient
	var patient account.Patient
	err := db.Where("account_id = ?", c.Get("id").(uint)).First(&patient).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", PatientNotFound))
	}

	var registration Registration
	err = db.Where("patient_id = ?", patient.ID).First(&registration, c.Param("registrationID")).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", RegistrationNotFound))
	}

	// get names
	var department Department
	var patientAccount account.Account
	var doctor account.Doctor
	var doctorAccount account.Account
	db.First(&department, registration.DepartmentID)
	db.First(&patientAccount, registration.PatientID)
	db.First(&doctor, registration.DoctorID)
	db.First(&doctorAccount, doctor.AccountID)

	registrationJSON := RegistrationDetailJSON{
		ID:              registration.ID,
		Department:      department.Name,
		Patient:         patientAccount.LastName + patientAccount.FirstName,
		Doctor:          doctorAccount.LastName + doctorAccount.FirstName,
		Year:            registration.Year,
		Month:           registration.Month,
		Day:             registration.Day,
		HalfDay:         registration.HalfDay,
		Status:          registration.Status,
		TerminatedCause: registration.TerminatedCause,
	}

	// get milestones
	var milestones []MileStone
	db.Where("registration_id = ?", registration.ID).Find(&milestones)
	registrationJSON.MileStone = milestones

	c.Logger().Debug("GetRegistrationByDoctor")
	return c.JSON(http.StatusOK, api.Return("ok", registrationJSON))
}

// GetRegistrationByDoctor
// @Summary get a registration by its ID (doctor view)
// @Tags Process
// @Description return a registration details by its ID
// @Produce json
// @Success 200 {object} api.ReturnedData{data=Registration}
// @Router /doctor/registration/{RegistrationID}/{DoctorID} [GET]
func (h *ProcessHandler) GetRegistrationByDoctor(c echo.Context) error {
	//// Must make sure RegistrationID in patientID's registration list.
	//if DoctorAccessToRegistration(c) {
	//	return c.JSON(http.StatusForbidden, api.Return("unauthorized", nil))
	//}
	//db := utils.GetDB()
	//var registration Registration
	//db.First(&registration, c.Param("RegistrationID"))
	//c.Logger().Debug("GetRegistrationByDoctor")
	//return c.JSON(http.StatusCreated, api.Return("ok", registration))
	db := utils.GetDB()

	// get doctor
	var doctor account.Doctor
	err := db.Where("account_id = ?", c.Get("id").(uint)).First(&doctor).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", DoctorNotFound))
	}

	var registration Registration
	err = db.Where("doctor_id = ?", doctor.ID).First(&registration, c.Param("registrationID")).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", RegistrationNotFound))
	}

	// get names
	var department Department
	var patientAccount account.Account
	var doctorAccount account.Account
	db.First(&department, registration.DepartmentID)
	db.First(&patientAccount, registration.PatientID)
	db.First(&doctorAccount, doctor.AccountID)

	registrationJSON := RegistrationDetailJSON{
		ID:              registration.ID,
		Department:      department.Name,
		Patient:         patientAccount.LastName + patientAccount.FirstName,
		Doctor:          doctorAccount.LastName + doctorAccount.FirstName,
		Year:            registration.Year,
		Month:           registration.Month,
		Day:             registration.Day,
		HalfDay:         registration.HalfDay,
		Status:          registration.Status,
		TerminatedCause: registration.TerminatedCause,
	}

	// get milestones
	var milestones []MileStone
	db.Where("registration_id = ?", registration.ID).Find(&milestones)
	registrationJSON.MileStone = milestones

	c.Logger().Debug("GetRegistrationByDoctor")
	return c.JSON(http.StatusOK, api.Return("ok", registrationJSON))
}

// UpdateRegistrationStatus
// @Summary update registration status
// @Tags Process
// @Description update registration status
// @Param registrationID path uint true "registration ID"
// @Param status body string true "next status of current registration"
// @Param terminatedCause body string false "if registration is shutdown, a cause is required"
// @Produce json
// @Success 200 {object} api.ReturnedData{}
// @Router /registration/{RegistrationID} [PUT]
func (h *ProcessHandler) UpdateRegistrationStatus(c echo.Context) error {

	type RegistrationSubmitJSON struct {
		RegistrationStatus       string `json:"status"`
		TerminatedCause			 string `json:"terminatedCause"`
	}

	// extract submit data
	var submit RegistrationSubmitJSON
	if err := c.Bind(&submit); err != nil {
		c.Logger().Debug("JSON format failed when trying to create a registration ...")
		return c.JSON(http.StatusBadRequest, api.Return("error", InvalidSubmitFormat))
	}

	db := utils.GetDB()
	// validation for status: select in frontend
	status := RegistrationStatusEnum(submit.RegistrationStatus)
	terminatedCause := submit.TerminatedCause
	var registration Registration
	err := db.First(&registration, c.Param("registrationID")).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}
	currentStatus := registration.Status
	registration.Status = status
	var acc account.Account
	err = db.Where("id = ?", c.Get("id").(uint)).First(&acc).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", AccountNotFound))
	}
	if currentStatus == committed {
		if acc.Type == account.PatientType {
			if status == terminated {
				registration.Status = status
				db.Save(&registration)
				return c.JSON(http.StatusOK, api.Return("ok", "修改挂号成功"))
			}
		}
		if acc.Type == account.DoctorType {
			if status == accepted {
				registration.Status = status
				db.Save(&registration)
				return c.JSON(http.StatusOK, api.Return("ok", "修改挂号成功"))
			}
			if status == terminated {
				if terminatedCause != "" {
					registration.Status = status
					registration.TerminatedCause = terminatedCause
					db.Save(&registration)
					return c.JSON(http.StatusOK, api.Return("ok", "修改挂号成功"))
				} else {
					return c.JSON(http.StatusBadRequest, api.Return("failed", "Missing terminated causes"))
				}
			}
		}
	}
	if currentStatus == accepted {
		if acc.Type == "doctor" {
			if status == terminated {
				registration.Status = status
				db.Save(&registration)
				return c.JSON(http.StatusOK, api.Return("ok", "修改挂号成功"))
			}
		}
	}
	return c.JSON(http.StatusBadRequest, api.Return("failed", RegistrationUpdateFailed))
}

// CreateMileStoneByDoctor
// @Summary create milestone
// @Tags Process
// @Description the doctor create milestone (type: array)
// @Param registrationID body uint true "registration's ID"
// @Param activity body string true "milestone's activity"
// @Produce json
// @Success 204 {string} api.ReturnedData{}
// @Router /milestone [POST]
func (h *ProcessHandler) CreateMileStoneByDoctor(c echo.Context) error {
	db := utils.GetDB()
	registrationID, err := strconv.ParseUint(c.QueryParam("registrationID"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}
	milestone := MileStone{
		RegistrationID: uint(registrationID),
		Activity:       c.QueryParam("activity"),
	}
	result := db.Create(&milestone)
	if result.Error != nil {
		return c.JSON(http.StatusUnprocessableEntity, api.Return("error", result.Error))
	}
	return c.JSON(http.StatusCreated, api.Return("ok", nil))
}

// UpdateMileStoneByDoctor
// @Summary update milestone
// @Tags Process
// @Description the doctor update milestone (check milestone)
// @Param mileStoneID path uint true "milestone's ID"
// @Param activity body string false "updated milestone's activity"
// @Param checked body boolean true "milestone is checked or not"
// @Produce json
// @Success 200 {string} api.ReturnedData{}
// @Router /milestone/{mileStoneID} [PUT]
func (h *ProcessHandler) UpdateMileStoneByDoctor(c echo.Context) error {
	db := utils.GetDB()
	var milestone MileStone
	var err error
	var checked bool

	db.First(&milestone, c.QueryParam("mileStoneID"))
	checked, err = strconv.ParseBool(c.Param("checked"))
	if err != nil {
		c.Logger().Error("UpdateMileStoneByDoctor failed due to", err)
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}
	milestone.Checked = checked
	milestone.Activity = c.QueryParam("activity")
	return c.JSON(http.StatusOK, api.Return("ok", nil))


}

// DeleteMileStoneByDoctor
// @Summary delete milestone
// @Tags Process
// @Description the doctor delete milestone
// @Param mileStoneID path uint true "milestone's ID"
// @Produce json
// @Success 200 {string} api.ReturnedData{}
// @Router /milestone/{mileStoneID} [DELETE]
func (h *ProcessHandler) DeleteMileStoneByDoctor(c echo.Context) error {
	db := utils.GetDB()

	// get doctor
	var doctor account.Doctor
	err := db.Where("account_id = ?", c.Get("id").(uint)).First(&doctor).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", DoctorNotFound))
	}

	// get mileStone
	var mileStone MileStone
	err = db.First(&mileStone, c.Param("mileStoneID")).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", MileStoneNotFound))
	}
	var registration Registration
	err = db.First(&registration, mileStone.RegistrationID).Error
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", RegistrationNotFound))
	}

	// check milestone authority
	if registration.Status == terminated || registration.DoctorID != doctor.ID {
		return c.JSON(http.StatusBadRequest, api.Return("error", MileStoneUnauthorized))
	}

	db.Delete(&mileStone)
	return c.JSON(http.StatusOK, api.Return("ok", nil))
}