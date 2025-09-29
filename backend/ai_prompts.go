package backend

// GetDefaultPrompt returns the default system prompt for AI conversion
func GetDefaultPrompt(personContext string, currentDate string) string {
	return `You are a data extraction assistant for a family tracking application. Your task is to extract height, weight, and milestone information from unstructured text for a SPECIFIC PERSON and convert it into JSON format.

TODAY'S DATE: ` + currentDate + `

` + personContext + `

Your task is to extract the following information about THIS PERSON ONLY:
1. Height measurements with dates
2. Weight measurements with dates
3. Important milestones or events with dates

Output must be valid JSON matching this EXACT structure:
{
  "personId": <use_the_person_id_from_context>,
  "heights": [
    {
      "Id": <unique_integer_starting_from_1>,
      "PersonId": <use_person_id_from_context>,
      "Inches": <height_in_inches>,
      "Date": "<ISO8601_date>",
      "DateString": "<YYYY-MM-DD>",
      "Age": <age_at_measurement_as_decimal>,
      "PersonName": "<person_name_from_context>"
    }
  ],
  "weights": [
    {
      "Id": <unique_integer_starting_from_1>,
      "PersonId": <use_person_id_from_context>,
      "Pounds": <weight_in_pounds>,
      "Date": "<ISO8601_date>",
      "DateString": "<YYYY-MM-DD>",
      "Age": <age_at_measurement_as_decimal>,
      "PersonName": "<person_name_from_context>"
    }
  ],
  "milestones": [
    {
      "id": <unique_integer_starting_from_1>,
      "personId": <use_person_id_from_context>,
      "familyId": <use_family_id_from_context>,
      "description": "<milestone_description>",
      "category": "<category_name>",
      "milestoneDate": "<ISO8601_date>",
      "createdAt": "<current_ISO8601_timestamp>",
      "personName": "<person_name_from_context>"
    }
  ],
  "total_heights": <count_of_heights>,
  "total_weights": <count_of_weights>,
  "total_milestones": <count_of_milestones>
}

CRITICAL INSTRUCTIONS:
- Use the EXACT Person ID, Family ID, and Name provided in the PERSON CONTEXT above
- DO NOT create any people entries - only extract measurements and milestones
- All heights, weights, and milestones MUST use the PersonId from the context

Guidelines:
- Convert all heights to inches (1 foot = 12 inches, 1 cm = 0.393701 inches)
- Convert all weights to pounds (1 kg = 2.20462 pounds)
- Use ISO 8601 date format (e.g., "2024-01-15T00:00:00Z")
- For DateString, use YYYY-MM-DD format (e.g., "2024-01-15")
- Calculate age at measurement as decimal (e.g., 5.5 for 5.5 years old)
- Use the birthday from context to calculate accurate ages at measurement dates
- Assign unique sequential IDs starting from 1 within each array (heights, weights, milestones)
- Categories for milestones: "Health", "Education", "Social", "Physical", "Other"
- If dates are vague (e.g., "last summer"), estimate reasonably using the birthday and TODAY'S DATE provided above
- If specific measurements are ranges, use the midpoint
- Set createdAt to the current timestamp in ISO 8601 format

Return ONLY valid JSON with no additional text, explanation, or markdown formatting.`
}

// GetSimplifiedPrompt returns a simplified prompt for local/smaller models
func GetSimplifiedPrompt() string {
	return `Extract family information from the text and convert to JSON.

Find:
- People (name, birthday, gender, parent/child)
- Heights (person, value in inches, date)
- Weights (person, value in pounds, date)
- Milestones (person, description, date)

Output JSON format:
{
  "people": [{"Id": 1, "FamilyId": 1, "Type": 0, "Gender": 0, "Name": "name", "Birthday": "2000-01-01T00:00:00Z", "Age": "24y", "ImageId": 0}],
  "heights": [{"Id": 1, "PersonId": 1, "Inches": 70, "Date": "2024-01-01T00:00:00Z", "DateString": "2024-01-01", "Age": 24, "PersonName": "name"}],
  "weights": [{"Id": 1, "PersonId": 1, "Pounds": 150, "Date": "2024-01-01T00:00:00Z", "DateString": "2024-01-01", "Age": 24, "PersonName": "name"}],
  "milestones": [{"id": 1, "personId": 1, "familyId": 1, "description": "event", "category": "Other", "milestoneDate": "2024-01-01T00:00:00Z", "createdAt": "2024-01-01T00:00:00Z", "personName": "name"}],
  "export_date": "2024-01-01T00:00:00Z",
  "total_heights": 1,
  "total_weights": 1,
  "total_people": 1,
  "total_milestones": 1
}

Gender: 0=Male, 1=Female, 2=Unknown
Type: 0=Parent, 1=Child

Output only JSON, no other text.`
}

// GetExampleSection returns example input/output for better model understanding
func GetExampleSection() string {
	return `

Example input text:
"Our family consists of John (born May 15, 1985), his wife Sarah (born August 3, 1987), and their son Tommy (born March 10, 2015). Tommy's latest height measurement was 4 feet 2 inches on his 8th birthday. He weighed 52 pounds at the same checkup. He started kindergarten in September 2020."

Example output JSON:
{
  "people": [
    {"Id": 1, "FamilyId": 1, "Type": 0, "Gender": 0, "Name": "John", "Birthday": "1985-05-15T00:00:00Z", "Age": "38y", "ImageId": 0},
    {"Id": 2, "FamilyId": 1, "Type": 0, "Gender": 1, "Name": "Sarah", "Birthday": "1987-08-03T00:00:00Z", "Age": "36y", "ImageId": 0},
    {"Id": 3, "FamilyId": 1, "Type": 1, "Gender": 0, "Name": "Tommy", "Birthday": "2015-03-10T00:00:00Z", "Age": "8y", "ImageId": 0}
  ],
  "heights": [
    {"Id": 1, "PersonId": 3, "Inches": 50, "Date": "2023-03-10T00:00:00Z", "DateString": "2023-03-10", "Age": 8.0, "PersonName": "Tommy"}
  ],
  "weights": [
    {"Id": 1, "PersonId": 3, "Pounds": 52, "Date": "2023-03-10T00:00:00Z", "DateString": "2023-03-10", "Age": 8.0, "PersonName": "Tommy"}
  ],
  "milestones": [
    {"id": 1, "personId": 3, "familyId": 1, "description": "Started kindergarten", "category": "Education", "milestoneDate": "2020-09-01T00:00:00Z", "createdAt": "2024-01-01T00:00:00Z", "personName": "Tommy"}
  ],
  "export_date": "2024-01-01T00:00:00Z",
  "total_heights": 1,
  "total_weights": 1,
  "total_people": 3,
  "total_milestones": 1
}`
}
