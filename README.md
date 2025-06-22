# Budget Tracker App 💰

A RESTful API built with Go 🦫 for managing personal budgets, expenses, and income categories.

---

## 📦 Requirements

- Go 1.20 or later
- MySQL database
- Internet connection
- A server (for production)

---

## ⚙️ Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/yourusername/budget-tracker.git
   cd budget-tracker
   ```

2. **Setup environment variables**
   ```bash
   cp env_sample .env
   ```
   Then open the **.env** file and fill in the required values such as your dbuser, dbpassword, dbname etc.
3. **Run the application**
   ```bash
   go run main.go
   ```
   **Note: You can define the port in the .env file. If you don't, port 8060 will be used by default.**

---

## 📑 View API Documentation

**To view the OpenAPI specification in Swagger Editor, follow these steps:**

1. **Copy the YAML code below**

```yaml
openapi: 3.0.0
info:
  title: Budget Tracker API
  description: REST API for user management, transactions, and categories.
  version: 1.0.0

components:
  schemas:
    Transaction:
      type: object
      properties:
        id:
          type: string
        category_name:
          type: string
        category_type:
          type: string
        amount:
          type: number
        currency:
          type: string
        created_at:
          type: string
          format: date-time
        note:
          type: string
        created_by:
          type: string

    IncomeCategory:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        amount:
          type: number
        target_amount:
          type: number
        usage_percent:
          type: number
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        note:
          type: string
        created_by:
          type: string

    ExpenseCategory:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        amount:
          type: number
        max_amount:
          type: number
        period_day:
          type: number
        is_expired:
          type: boolean
        usage_percent:
          type: number
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        note:
          type: string
        created_by:
          type: string

  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer

paths:
  /register:
    post:
      summary: Register a new user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                username:
                  type: string
                  example: "john_doe"
                fullname:
                  type: string
                  example: "John K. Doe"
                email:
                  type: string
                  example: "john.doe@gmail.com"
                password:
                  type: string
                  example: "doe2004"
      responses:
        "201":
          description: User created
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Registration completed successfully
                  Extra:
                    type: string
                    example: eyJhbGciOiJIUzI1NiIsInR5cCI6
              example:
                Code: SUCCESS
                Message: Registration completed successfully
                Extra: eyJhb6(token)

  /login:
    post:
      summary: Login a user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                username:
                  type: string
                  example: "john_doe"
                password:
                  type: string
                  example: "doe2004"
      responses:
        "200":
          description: User logged in
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Welcome!
                  Extra:
                    type: string
                    example: eyJhbGciOiJIUzI1NiIsInR5cCI6
              example:
                Code: SUCCESS
                Message: Welcome!
                Extra: eyJhb6(token)

  /logout:
    get:
      summary: Logout the user
      security:
        - BearerAuth: []
      responses:
        "200":
          description: User logged out
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Bye!

  /remove-account:
    post:
      summary: Remove user account
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                password:
                  type: string
                  example: "doe2004"
                reason:
                  type: string
                  example: "too complex"
      responses:
        "200":
          description: User account removed
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Account deleted successfully
              example:
                Code: SUCCESS
                Message: Account deleted successfully

  /transaction:
    post:
      summary: Create a transaction
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                category_name:
                  type: string
                  example: "home repair"
                category_type:
                  type: string
                  example: "-"
                amount:
                  type: number
                  example: 314.5
                currency:
                  type: string
                  example: "USD"
                note:
                  type: string
                  example: "doors fixed"
      responses:
        "201":
          description: Transaction posted
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Transaction posted
              example:
                Code: SUCCESS
                Message: Transaction posted
    get:
      summary: Get filtered transactions
      security:
        - BearerAuth: []
      parameters:
        - in: query
          name: category_names
          required: true
          schema:
            type: string
            example: salary, freelance, business
        - in: query
          name: amount
          schema:
            type: number
            example: 400
        - in: query
          name: currency
          schema:
            type: string
            example: USD
        - in: query
          name: created_at
          schema:
            type: string
            example: 21/06/2025
        - in: query
          name: category_type
          required: true
          schema:
            type: string
            example: income
      responses:
        "200":
          description: List of transactions
          content:
            application/json:
              schema:
                type: object
                properties:
                  Transactions:
                    type: array
                    items:
                      $ref: "#/components/schemas/Transaction"

  /transaction/{id}:
    get:
      summary: Get transaction by ID
      security:
        - BearerAuth: []
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Transaction detail
          content:
            application/json:
              schema:
                type: object
                properties:
                  Transaction:
                    $ref: "#/components/schemas/Transaction"

  /image-process:
    post:
      summary: Process an image for transaction extraction
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                image:
                  type: string
                  format: binary
      responses:
        "200":
          description: Extracted transaction fields
          content:
            application/json:
              schema:
                type: object
                properties:
                  amounts:
                    type: string
                    example: 34.3, 124, 451
                  currencies_iso:
                    type: string
                    example: "USD, AZN, EUR, JPY"
                  currenciesSymbol:
                    type: string
                    example: "$, €"

  /category/expense:
    post:
      summary: Create an expense category
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  example: health
                max_amount:
                  type: number
                  example: 1000
                period_day:
                  type: number
                  example: 7
                note:
                  type: string
                  example: toe nail surgoen, check-up for men.
      responses:
        "201":
          description: Category created
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Category created successfully
    get:
      summary: Get filtered expense categories
      security:
        - BearerAuth: []
      parameters:
        - in: query
          name: names
          required: true
          schema:
            type: string
            example: home restore, clothes
        - in: query
          name: max_amount
          schema:
            type: number
            example: 500
        - in: query
          name: period_day
          schema:
            type: number
            example: 7
        - in: query
          name: created_at
          schema:
            type: string
            example: 21/06/2025
        - in: query
          name: end_date
          schema:
            type: string
            example: 21/06/2026

      responses:
        "200":
          description: List of expense categories
          content:
            application/json:
              schema:
                type: object
                properties:
                  Categories:
                    type: array
                    items:
                      $ref: "#/components/schemas/ExpenseCategory"

    put:
      summary: Update expense category
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                id:
                  type: string
                new_name:
                  type: string
                new_max_amount:
                  type: number
                new_period_day:
                  type: number
                new_note:
                  type: string
      responses:
        "200":
          description: Category updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ExpenseCategory"

  /category/expense/{id}:
    delete:
      summary: Delete expense category
      security:
        - BearerAuth: []
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: Category delete
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Category deleted successfully

  /category/income:
    post:
      summary: Create an income category
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  example: e-commerce
                target_amount:
                  type: number
                  example: 1000
                note:
                  type: string
                  example: new QR-Code readers from Estonia
      responses:
        "201":
          description: Category created
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Category created successfully
    get:
      summary: Get filtered income categories
      security:
        - BearerAuth: []
      parameters:
        - in: query
          name: names
          required: true
          schema:
            type: string
            example: e-commerce, freelance, salary
        - in: query
          name: target_amount
          schema:
            type: string
            example: 500
        - in: query
          name: created_at
          schema:
            type: string
            example: 21/06/2025
        - in: query
          name: end_date
          schema:
            type: string
            example: 21/06/2026
      responses:
        "200":
          description: List of income categories
          content:
            application/json:
              schema:
                type: object
                properties:
                  Categories:
                    type: array
                    items:
                      $ref: "#/components/schemas/IncomeCategory"

    put:
      summary: Update income category
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                id:
                  type: string
                new_name:
                  type: string
                new_target_amount:
                  type: number
                new_note:
                  type: string
      responses:
        "200":
          description: Category updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/IncomeCategory"

  /category/income/{id}:
    delete:
      summary: Delete income category
      security:
        - BearerAuth: []
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: Category deleted
          content:
            application/json:
              schema:
                type: object
                properties:
                  Code:
                    type: string
                    example: SUCCESS
                  Message:
                    type: string
                    example: Category deleted successfully
```

2. **Open the Swagger editor by visiting: https://editor.swagger.io**

3. **Paste the copied YAML code into the editor.**

---

## Features

- **User registration, login, and logout**

- **Create, read, update, delete (CRUD) for both categories**

- **Expense and income categories**

- **OCR support: upload images of receipts to auto-fill transaction fields**

- **CORS-ready for frontend integration**
