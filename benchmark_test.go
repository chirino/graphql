package graphql_test

import (
	"testing"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/internal/example/starwars"
	"github.com/stretchr/testify/require"
)

func BenchmarkStarwarsQuery(b *testing.B) {
	engine := graphql.New()
	engine.Root = &starwars.Resolver{}
	err := engine.Schema.Parse(starwars.Schema)
	require.NoError(b, err)

	// Lets build a query that throws the kitchen skink at the query engine.
	// (we grab a little bit of all the tests we have so far)
	request := &graphql.Request{
		OperationName: "",
		Query: `
	query HeroNameAndFriends($episode: Episode, $withoutFriends: Boolean!, $withFriends: Boolean!) {
		hero {
			id
			name
			friends {
				name
			}
		}
		empireHerhero: hero(episode: EMPIRE) {
			name
		}
		jediHero: hero(episode: JEDI) {
			name
		}
		human(id: "1000") {
			name
			height(unit: FOOT)
		}
		leftComparison: hero(episode: EMPIRE) {
			...comparisonFields
			...height
		}
		rightComparison: hero(episode: JEDI) {
			...comparisonFields
			...height
		}
		heroNameAndFriends:	hero(episode: $episode) {
			name
		}
		heroSkip: hero(episode: $episode) {
			name
			friends @skip(if: $withoutFriends) {
				name
			}
		}
			
		heroInclude:  hero(episode: $episode) {
			name
			...friendsFragment @include(if: $withFriends)
		}
		inlineFragments: hero(episode: $episode) {
			name
			... on Droid {
				primaryFunction
			}
			... on Human {
				height
			}
		}
		search(text: "an") {
			__typename
			... on Human {
				name
			}
			... on Droid {
				name
			}
			... on Starship {
				name
			}
		}
		heroConnections: hero {
			name
			friendsConnection {
				totalCount
				pageInfo {
					startCursor
					endCursor
					hasNextPage
				}
				edges {
					cursor
					node {
						name
					}
				}
			}
		}
		reviews(episode: JEDI) {
			stars
			commentary
		}
		__schema {
			types {
				name
			}
		}
		__type(name: "Droid") {
			name
			fields {
				name
				args {
					name
					type {
						name
					}
					defaultValue
				}
				type {
					name
					kind
				}
			}
		}	
	}

	fragment comparisonFields on Character {
		name
		appearsIn
		friends {
			name
		}
	}
	fragment height on Human {
		height
	}
	fragment friendsFragment on Character {
		friends {
			name
		}
	}	
	`,
		Variables: map[string]interface{}{
			"episode":        "JEDI",
			"withoutFriends": true,
			"withFriends":    false,
			"review": map[string]interface{}{
				"stars":      5,
				"commentary": "This is a great movie!",
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			engine.ServeGraphQL(request)
		}
	})
}
