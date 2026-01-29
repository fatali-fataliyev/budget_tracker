# Budget Tracker App 💰(v.2.1.0)

A RESTful API built with Go 🦫 for managing personal budgets, expenses, and incomes, extract data from receipt.

---

#### Frontend repo: https://github.com/fatali-fataliyev/budget_tracker_frontend

---

## 📦 Requirements

- Go 1.24 or later
- MySQL database
- Internet connection(for OCR_API)
- Docker(optional)

---

## 🔧 Manual Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/fatali-fataliyev/budget_tracker
   cd budget-tracker
   ```

2. **Setup environment variables**
   ```bash
   cp env_sample .env
   ```
   Then open the **.env** file and fill in the required values such as your _db user_, _db host_, _db password_, _dbname_ etc.
3. **Run the application**
   ```bash
   go run main.go
   ```

---

## ⚙️ Docker Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/fatali-fataliyev/budget_tracker
   cd budget-tracker
   ```

2. **Setup environment variables**
   ```bash
   cp env_sample .env
   ```
3. **Execute dockerization command**
   ```bash
   docker-compose up --build
   ```

---

## 📑 View API Documentation

1. **Copy the yaml file below**

<details>
  <summary><strong>Click to expand and copy the OpenAPI YAML</strong></summary>

```yaml
openapi: 3.0.0
info:
  title: Budget Tracker API
  description: REST API for user management, transactions, and categories.
  version: 2.0.0

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
  api/register:
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

  api/login:
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

  api/logout:
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

  api/remove-account:
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

  api/download-user-data:
    post:
      summary: Download user data as zip
      security:
        - BearerAuth: []
      responses:
        "200":
          description: Zip file response.
  api/account:
    get:
      summary: Get user information
      security:
        - BearerAuth: []
      responses:
        "200":
          description: User info
          content:
            application/json:
              schema:
                type: object
                properties:
                  username:
                    type: string
                    example: john_doe
                  fullname:
                    type: string
                    example: John K. Doe
                  email:
                    type: string
                    example: johndoe@gmail.com
                  joined_at:
                    type: string
                    example: 2025-06-28 18:19:49

  api/transaction:
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

  api/transaction/{id}:
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

  api/image-process:
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

  api/category/expense:
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

  api/category/expense/{id}:
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

  api/category/income:
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

  api/category/income/{id}:
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

  api/statistics/expense:
    get:
      summary: Get statistics of expense categories
      security:
        - BearerAuth: []
      responses:
        "200":
          description: Statistics
          content:
            application/json:
              schema:
                type: object
                properties:
                  more_than_1000:
                    type: number
                    example: 12
                  between_500_and_1000:
                    type: number
                    example: 13
                  less_than_500:
                    type: number
                    example: 14

  api/statistics/income:
    get:
      summary: Get statistics of income categories
      security:
        - BearerAuth: []
      responses:
        "200":
          description: Statistics
          content:
            application/json:
              schema:
                type: object
                properties:
                  more_than_1000:
                    type: number
                    example: 12
                  between_500_and_1000:
                    type: number
                    example: 13
                  less_than_500:
                    type: number
                    example: 14

  api/statistics/transaction:
    get:
      summary: Get statistics of transactions
      security:
        - BearerAuth: []
      responses:
        "200":
          description: Statistics
          content:
            application/json:
              schema:
                type: object
                properties:
                  total:
                    type: number
                    example: 100
                  incomes:
                    type: number
                    example: 50
                  expenses:
                    type: number
                    example: 50
```

</details>

2. **Open the [link](https://editor.swagger.io), paste the editor.**
